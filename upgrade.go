package contracts

import (
	"bytes"
	"encoding/json"
	"fmt"
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
