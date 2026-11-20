package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// An upgrade step is Upgrade<Kind>(from []byte) ([]byte, error), one per version step, tested by the pairs in testdata/upgrade/<dir>/<from>-<to>/ (CT-RULE-3).

// reasonOf is the owner's mapping of an error/v1 code to a Problem's type and reason, the one mankit serve writes; a code not here stays the type, with no reason.
var reasonOf = map[string][2]string{
	"token_refused":     {"refused", "token"},
	"host_refused":      {"refused", "host"},
	"origin_refused":    {"refused", "origin"},
	"not_json":          {"refused", "not_json"},
	"unconfirmed":       {"refused", "unconfirmed"},
	"config_unreadable": {"failed", "config_unreadable"},
	"core_needs_values": {"failed", "core_needs_values"},
	// mankit's and the apps' other words, by the same pattern: failed when the request was fine and the app could not do it, refused when the request is not taken.
	"feature_needs_values":      {"failed", "feature_needs_values"},
	"queue_timeout":             {"failed", "queue_timeout"},
	"not_read_back":             {"failed", "not_read_back"},
	"ticket_lost":               {"failed", "ticket_lost"},
	"queue_unusable":            {"failed", "queue_unusable"},
	"queue_backend_unsupported": {"failed", "queue_backend_unsupported"},
	"change_too_long":           {"failed", "change_too_long"},
	"too_many_streams":          {"refused", "too_many_streams"},
	"too_large":                 {"refused", "too_large"},
}

type errorV1 struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	At      *struct {
		File  string `json:"file"`
		Field string `json:"field"`
	} `json:"at"`
}

const problemAPIVersion = "elderlygate/contracts/problem/v1"

type place struct {
	File string `json:"file,omitempty"`
	Path string `json:"path,omitempty"`
}

type problem struct {
	APIVersion string  `json:"apiVersion"`
	Kind       string  `json:"kind"`
	Type       string  `json:"type"`
	Detail     string  `json:"detail"`
	Reason     string  `json:"reason,omitempty"`
	At         []place `json:"at,omitempty"`
}

// UpgradeEnvelope is the step from envelope/v1 to envelope/v2: ran becomes source unchanged, and each error/v1 item becomes a Problem.
// A code becomes type and reason by the owner's mapping (any other code stays the type), message becomes detail, and at's file and field become one place's file and path.
// Dropped: at's kind and id, as a place has no field for them; at's file when problem/v1 refuses it as a machine path, rather than refusing the whole answer; at itself when nothing is left; and supported, as the version answer lists the versions.
// The app's own apiVersion, kind and data are untouched: they name its contract, not the envelope.
// It is Go only for now, as the readers that mix versions (servers, control-room) are Go; a TypeScript step comes only if a page needs one.
func UpgradeEnvelope(v1 []byte) ([]byte, error) {
	if err := Validate("envelope/v1/envelope.schema.json", v1); err != nil {
		return nil, fmt.Errorf("not an envelope/v1 answer: %w", err)
	}
	var in struct {
		APIVersion string            `json:"apiVersion"`
		Kind       string            `json:"kind"`
		Ran        json.RawMessage   `json:"ran"`
		Data       json.RawMessage   `json:"data"`
		Errors     []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(v1, &in); err != nil {
		return nil, err
	}
	out := struct {
		APIVersion string          `json:"apiVersion"`
		Kind       string          `json:"kind"`
		Source     json.RawMessage `json:"source"`
		Data       json.RawMessage `json:"data"`
		Errors     []problem       `json:"errors"`
	}{APIVersion: in.APIVersion, Kind: in.Kind, Source: in.Ran, Data: in.Data, Errors: []problem{}}
	for _, raw := range in.Errors {
		var e errorV1
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, err
		}
		p := problem{APIVersion: problemAPIVersion, Kind: "Problem", Type: e.Code, Detail: e.Message}
		if m, ok := reasonOf[e.Code]; ok {
			p.Type, p.Reason = m[0], m[1]
		}
		if e.At != nil {
			at := place{File: e.At.File, Path: e.At.Field}
			if at.File != "" && !takenFile(at.File) {
				at.File = ""
			}
			if at != (place{}) {
				p.At = []place{at}
			}
		}
		out.Errors = append(out.Errors, p)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	if err := Validate("envelope/v2/envelope.schema.json", buf.Bytes()); err != nil {
		return nil, fmt.Errorf("the upgrade is not an envelope/v2 answer: %w", err)
	}
	return buf.Bytes(), nil
}

// takenFile is whether problem/v1 takes file as a place's file, so the rule is problem/v1's alone.
func takenFile(file string) bool {
	doc, err := json.Marshal(problem{APIVersion: problemAPIVersion, Kind: "Problem", Type: "x", Detail: "x", At: []place{{File: file}}})
	return err == nil && Validate("problem/v1/problem.schema.json", doc) == nil
}

const (
	designerV1      = "elderlygate/designer/app/v1"
	designerV2      = "elderlygate/designer/app/v2"
	designFormV2    = "elderlygate/designer/design/v2"
	designFormV3    = "elderlygate/designer/design/v3"
	errorAPIVersion = "elderlygate/contracts/error/v1"
)

// UpgradeDesigner is the step from designer/app/v1 to v2: the envelope step, then the app's own form and contract, the design files' form inside the data (design/v2 to design/v3), and for Config the readings v2 keeps and the styles, colours and rules v1 never had, given empty.
// A v1 Version answer has no v2 kind: every app's version answer is contracts version/v1.
func UpgradeDesigner(v1 []byte) ([]byte, error) {
	var head struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(v1, &head); err != nil {
		return nil, err
	}
	if head.APIVersion != designerV1 {
		return nil, fmt.Errorf("not a designer/app/v1 answer: apiVersion is %q", head.APIVersion)
	}
	if head.Kind == "Version" {
		return nil, fmt.Errorf("designer/app/v2 has no Version: the version answer is contracts version/v1")
	}
	if err := Validate(DesignerSchema("v1", head.Kind), v1); err != nil {
		return nil, fmt.Errorf("not a designer/app/v1 %s: %w", head.Kind, err)
	}
	enveloped, err := UpgradeEnvelope(v1)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(enveloped, &doc); err != nil {
		return nil, err
	}
	doc["apiVersion"] = designerV2
	doc["source"].(map[string]any)["contract"] = "v2"
	doc["data"] = swapForm(doc["data"], designFormV2, designFormV3)
	if head.Kind == "Config" {
		upConfig(doc["data"].(map[string]any))
	}
	return encodeChecked(doc, DesignerSchema("v2", head.Kind))
}

// DowngradeDesigner is the step back from designer/app/v2 to v1, for a v1 reader (ADR-S16): the envelope back to ran and error/v1, the app's own form and contract, the design files' form back to design/v2, and for Config the readings v1 asks for, worked out by v1's own rules.
// What v1 cannot hold is said in a converted_down error naming each part lost, so a v1 reader knows its answer is not whole.
func DowngradeDesigner(v2 []byte) ([]byte, error) {
	var head struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(v2, &head); err != nil {
		return nil, err
	}
	if head.APIVersion != designerV2 {
		return nil, fmt.Errorf("not a designer/app/v2 answer: apiVersion is %q", head.APIVersion)
	}
	if err := Validate(DesignerSchema("v2", head.Kind), v2); err != nil {
		return nil, fmt.Errorf("not a designer/app/v2 %s: %w", head.Kind, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(v2, &doc); err != nil {
		return nil, err
	}
	source := doc["source"].(map[string]any)
	source["contract"] = "v1"
	out := map[string]any{"apiVersion": designerV1, "kind": head.Kind, "ran": source}
	data := swapForm(doc["data"], designFormV3, designFormV2)
	var lost []string
	if head.Kind == "Config" {
		lost = downConfig(data.(map[string]any))
	}
	out["data"] = data
	errs := []any{}
	for _, raw := range doc["errors"].([]any) {
		errs = append(errs, errorOf(raw.(map[string]any)))
	}
	if len(lost) > 0 {
		errs = append(errs, map[string]any{
			"apiVersion": errorAPIVersion, "kind": "Error", "code": "converted_down",
			"message": "This answer was converted down from designer/app/v2, which v1 cannot hold whole; lost: " + strings.Join(lost, ", "),
		})
	}
	out["errors"] = errs
	return encodeChecked(out, DesignerSchema("v1", head.Kind))
}

// upConfig keeps the readings designer/app/v2 has, and gives the styles, colours and rules a v1 answer never said as empty, never guessed.
func upConfig(data map[string]any) {
	if readings, ok := data["readings"].(map[string]any); ok {
		delete(readings, "borderStyles")
		delete(readings, "lineStyles")
		if statuses, ok := readings["statuses"].([]any); ok {
			for _, s := range statuses {
				st := s.(map[string]any)
				for _, look := range []string{"mark", "badge", "color", "recede"} {
					delete(st, look)
				}
			}
		}
	}
	data["styles"] = map[string]any{}
	data["colors"] = map[string]any{}
	data["rules"] = []any{}
}

// downConfig gives the readings designer/app/v1 asks for by its own rules (no style words; a status's badge its first letter in upper case) and takes out what v1 cannot hold, naming it.
func downConfig(data map[string]any) []string {
	var lost []string
	for _, part := range []string{"styles", "colors", "rules"} {
		if said(data[part]) {
			lost = append(lost, part)
		}
		delete(data, part)
	}
	if readings, ok := data["readings"].(map[string]any); ok {
		readings["borderStyles"] = []any{}
		readings["lineStyles"] = []any{}
		if statuses, ok := readings["statuses"].([]any); ok {
			for _, s := range statuses {
				st := s.(map[string]any)
				if value, _ := st["value"].(string); value != "" {
					first, _ := utf8.DecodeRuneInString(value)
					st["badge"] = strings.ToUpper(string(first))
				}
			}
		}
	}
	return lost
}

// said is whether a part holds anything, as an empty one loses nothing when it goes.
func said(v any) bool {
	switch t := v.(type) {
	case map[string]any:
		return len(t) > 0
	case []any:
		return len(t) > 0
	}
	return v != nil
}

// errorOf turns a Problem back into an error/v1 item: the owner's mapping read backwards, the detail as the message, the first place as at.
func errorOf(p map[string]any) map[string]any {
	code, _ := p["type"].(string)
	reason, _ := p["reason"].(string)
	for c, m := range reasonOf {
		if m[0] == code && m[1] == reason {
			code = c
			break
		}
	}
	e := map[string]any{"apiVersion": errorAPIVersion, "kind": "Error", "code": code, "message": p["detail"]}
	if places, ok := p["at"].([]any); ok && len(places) > 0 {
		pl := places[0].(map[string]any)
		at := map[string]any{}
		if f, ok := pl["file"]; ok {
			at["file"] = f
		}
		if f, ok := pl["path"]; ok {
			at["field"] = f
		}
		e["at"] = at
	}
	return e
}

// swapForm changes every apiVersion in the data that names the design form from to the form to, as a design file inside an answer is written in its contract's form.
func swapForm(v any, from, to string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if k == "apiVersion" && x == from {
				t[k] = to
				continue
			}
			t[k] = swapForm(x, from, to)
		}
	case []any:
		for i, x := range t {
			t[i] = swapForm(x, from, to)
		}
	}
	return v
}

// DesignerSchema is the schema of a designer answer kind, its file named in kebab case like the kind.
func DesignerSchema(version, kind string) string {
	var b strings.Builder
	for i, r := range kind {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('-')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return "designer/" + version + "/" + b.String() + ".schema.json"
}

// encodeChecked writes doc as the steps write every answer, and checks it meets schema, so a step never hands on an answer its version refuses.
func encodeChecked(doc any, schema string) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := Validate(schema, buf.Bytes()); err != nil {
		return nil, fmt.Errorf("the step does not give a %s answer: %w", schema, err)
	}
	return buf.Bytes(), nil
}
