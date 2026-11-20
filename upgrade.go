package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// An upgrade step is Upgrade<Kind>(from []byte) ([]byte, error), one per version step, and its down step Downgrade<Kind>To<v>(from []byte) ([]byte, []string, error); each is tested by the pairs in testdata/upgrade/<dir>/<from>-<to>/ (CT-RULE-3).

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
	b, err := encode(out)
	if err != nil {
		return nil, err
	}
	if err := Validate("envelope/v2/envelope.schema.json", b); err != nil {
		return nil, fmt.Errorf("the upgrade is not an envelope/v2 answer: %w", err)
	}
	return b, nil
}

// takenFile is whether problem/v1 takes file as a place's file, so the rule is problem/v1's alone.
func takenFile(file string) bool {
	doc, err := json.Marshal(problem{APIVersion: problemAPIVersion, Kind: "Problem", Type: "x", Detail: "x", At: []place{{File: file}}})
	return err == nil && Validate("problem/v1/problem.schema.json", doc) == nil
}

// Each version step converts both ways in one place (ADR-S16): Upgrade<Kind>To<v> makes the newer form, Downgrade<Kind>To<v> the older one and says, as paths such as problems.0, what the older form could not hold and was left out. Both are tested by pairs in testdata/upgrade/<dir>/<from>-<to>/.

// encode is out as a step writes it: indented by two, with <, > and & kept for people.
func encode(out any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// envelopeV3 is an envelope/v3 answer; v2 is the same without problems.
type envelopeV3 struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Source     json.RawMessage   `json:"source"`
	Data       json.RawMessage   `json:"data"`
	Errors     []json.RawMessage `json:"errors"`
	Problems   []json.RawMessage `json:"problems"`
}

type envelopeV2 struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Source     json.RawMessage   `json:"source"`
	Data       json.RawMessage   `json:"data"`
	Errors     []json.RawMessage `json:"errors"`
}

// UpgradeEnvelopeToV3 is the step from envelope/v2 to envelope/v3: everything stays, and problems is there and empty, as an envelope/v2 answer worked nothing around that it could say.
func UpgradeEnvelopeToV3(v2 []byte) ([]byte, error) {
	if err := Validate("envelope/v2/envelope.schema.json", v2); err != nil {
		return nil, fmt.Errorf("not an envelope/v2 answer: %w", err)
	}
	var in envelopeV2
	if err := json.Unmarshal(v2, &in); err != nil {
		return nil, err
	}
	out, err := encode(envelopeV3{APIVersion: in.APIVersion, Kind: in.Kind, Source: in.Source, Data: in.Data, Errors: nonNil(in.Errors), Problems: []json.RawMessage{}})
	if err != nil {
		return nil, err
	}
	if err := Validate("envelope/v3/envelope.schema.json", out); err != nil {
		return nil, fmt.Errorf("the upgrade is not an envelope/v3 answer: %w", err)
	}
	return out, nil
}

// DowngradeEnvelopeToV2 is the step from envelope/v3 back to envelope/v2: everything stays but problems, which envelope/v2 has no place for, each one lost.
func DowngradeEnvelopeToV2(v3 []byte) ([]byte, []string, error) {
	if err := Validate("envelope/v3/envelope.schema.json", v3); err != nil {
		return nil, nil, fmt.Errorf("not an envelope/v3 answer: %w", err)
	}
	var in envelopeV3
	if err := json.Unmarshal(v3, &in); err != nil {
		return nil, nil, err
	}
	out, err := encode(envelopeV2{APIVersion: in.APIVersion, Kind: in.Kind, Source: in.Source, Data: in.Data, Errors: nonNil(in.Errors)})
	if err != nil {
		return nil, nil, err
	}
	if err := Validate("envelope/v2/envelope.schema.json", out); err != nil {
		return nil, nil, fmt.Errorf("the downgrade is not an envelope/v2 answer: %w", err)
	}
	return out, indexed("problems", len(in.Problems)), nil
}

// versionV1 and versionV2 are the data of the two version answers.
type versionV1 struct {
	SupportedContracts []string `json:"supportedContracts"`
}

type versionV2 struct {
	Reads  []string `json:"reads"`
	Writes []string `json:"writes"`
}

// UpgradeVersionToV2 is the step from version/v1 to version/v2: the envelope steps up from v1 to v3, the versions it served become reads, and the newest two of them writes, as an app before version/v2 took its own version and the one before.
func UpgradeVersionToV2(v1 []byte) ([]byte, error) {
	if err := Validate("version/v1/version.schema.json", v1); err != nil {
		return nil, fmt.Errorf("not a version/v1 answer: %w", err)
	}
	v2, err := UpgradeEnvelope(v1)
	if err != nil {
		return nil, err
	}
	v3, err := UpgradeEnvelopeToV3(v2)
	if err != nil {
		return nil, err
	}
	var env envelopeV3
	if err := json.Unmarshal(v3, &env); err != nil {
		return nil, err
	}
	var data versionV1
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return nil, err
	}
	writes := data.SupportedContracts[max(0, len(data.SupportedContracts)-2):]
	if env.Data, err = json.Marshal(versionV2{Reads: data.SupportedContracts, Writes: writes}); err != nil {
		return nil, err
	}
	env.APIVersion = "elderlygate/contracts/version/v2"
	out, err := encode(env)
	if err != nil {
		return nil, err
	}
	if err := Validate("version/v2/version.schema.json", out); err != nil {
		return nil, fmt.Errorf("the upgrade is not a version/v2 answer: %w", err)
	}
	return out, nil
}

// DowngradeVersionToV1 is the step from version/v2 back to version/v1, which stays on envelope/v1: source becomes ran, the versions it reads become the ones it served, and writes and the problems, which version/v1 has no place for, are lost.
// An answer with errors is refused, as the version answer never fails (ADR-S03) and envelope/v1 errors are a form apart.
func DowngradeVersionToV1(v2 []byte) ([]byte, []string, error) {
	if err := Validate("version/v2/version.schema.json", v2); err != nil {
		return nil, nil, fmt.Errorf("not a version/v2 answer: %w", err)
	}
	var in envelopeV3
	if err := json.Unmarshal(v2, &in); err != nil {
		return nil, nil, err
	}
	if len(in.Errors) > 0 {
		return nil, nil, fmt.Errorf("a version/v2 answer with errors has no version/v1 form: the version answer never fails")
	}
	var data versionV2
	if err := json.Unmarshal(in.Data, &data); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(versionV1{SupportedContracts: data.Reads})
	if err != nil {
		return nil, nil, err
	}
	out, err := encode(struct {
		APIVersion string            `json:"apiVersion"`
		Kind       string            `json:"kind"`
		Ran        json.RawMessage   `json:"ran"`
		Data       json.RawMessage   `json:"data"`
		Errors     []json.RawMessage `json:"errors"`
	}{APIVersion: "elderlygate/contracts/version/v1", Kind: in.Kind, Ran: in.Source, Data: body, Errors: []json.RawMessage{}})
	if err != nil {
		return nil, nil, err
	}
	if err := Validate("version/v1/version.schema.json", out); err != nil {
		return nil, nil, fmt.Errorf("the downgrade is not a version/v1 answer: %w", err)
	}
	return out, append([]string{"data.writes"}, indexed("problems", len(in.Problems))...), nil
}

// nonNil is list, or an empty one in place of none, as an envelope's lists are always there.
func nonNil(list []json.RawMessage) []json.RawMessage {
	if list == nil {
		return []json.RawMessage{}
	}
	return list
}

// indexed is the path of each of n items of the list at field, such as problems.0.
func indexed(field string, n int) []string {
	out := []string{}
	for i := range n {
		out = append(out, fmt.Sprintf("%s.%d", field, i))
	}
	return out
}
