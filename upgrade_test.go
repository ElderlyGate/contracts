package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ElderlyGate/contracts"
)

// steps is every step by its folder in testdata/upgrade, with the schema of the version it reads and the version it makes, each by the answer's kind; a step down (v2-v1) reads the .v2.json of a pair and makes its .v1.json.
var steps = map[string]struct {
	step     func([]byte) ([]byte, error)
	from, to func(kind string) string
	down     bool
}{
	"envelope/v1-v2": {contracts.UpgradeEnvelope, fixed("envelope/v1/envelope.schema.json"), fixed("envelope/v2/envelope.schema.json"), false},
	"designer/v1-v2": {contracts.UpgradeDesigner, designerSchema("v1"), designerSchema("v2"), false},
	"designer/v2-v1": {contracts.DowngradeDesigner, designerSchema("v2"), designerSchema("v1"), true},
}

func fixed(schema string) func(string) string { return func(string) string { return schema } }

func designerSchema(version string) func(string) string {
	return func(kind string) string { return contracts.DesignerSchema(version, kind) }
}

// stepOf is the step of a file in testdata/upgrade/<dir>/<from>-<to>/, or a failure naming the folder with no step.
func stepOf(t *testing.T, path string) (func([]byte) ([]byte, error), func(string) string, func(string) string, bool) {
	t.Helper()
	folder := filepath.ToSlash(strings.TrimPrefix(filepath.Dir(path), filepath.Join("testdata", "upgrade")+string(filepath.Separator)))
	s, ok := steps[folder]
	if !ok {
		t.Fatalf("no upgrade step for %s", folder)
	}
	return s.step, s.from, s.to, s.down
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decode(t *testing.T, b []byte) any {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, b)
	}
	return v
}

// Each pair in testdata/upgrade/<dir>/<from>-<to>/ is an answer of the version a step reads and the answer the step makes of it, each meeting its own version (CT-RULE-3, ADR-S16).
func TestUpgradeStepsMatchTheirSamples(t *testing.T) {
	pairs, err := filepath.Glob("testdata/upgrade/*/*/*.v1.json")
	if err != nil || len(pairs) == 0 {
		t.Fatalf("no upgrade pairs found: %v", err)
	}
	for _, v1 := range pairs {
		t.Run(v1, func(t *testing.T) {
			step, fromSchema, toSchema, down := stepOf(t, v1)
			v2 := strings.TrimSuffix(v1, ".v1.json") + ".v2.json"
			from, to := v1, v2
			if down {
				from, to = v2, v1
			}
			older, want := readFile(t, from), readFile(t, to)
			kind := kindOf(t, older)
			if err := contracts.Validate(fromSchema(kind), older); err != nil {
				t.Fatalf("the sample read does not meet %s: %v", fromSchema(kind), err)
			}
			if err := contracts.Validate(toSchema(kind), want); err != nil {
				t.Fatalf("the sample made does not meet %s: %v", toSchema(kind), err)
			}
			got, err := step(older)
			if err != nil {
				t.Fatal(err)
			}
			// A detail is read by people, so <, > and & stay as they are.
			if strings.Contains(string(got), `\u003c`) || strings.Contains(string(got), `\u0026`) {
				t.Fatalf("the step escaped text meant for people:\n%s", got)
			}
			if !reflect.DeepEqual(decode(t, got), decode(t, want)) {
				t.Fatalf("the step gave\n%s\nwant\n%s", got, want)
			}
		})
	}
}

func kindOf(t *testing.T, b []byte) string {
	t.Helper()
	var head struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		t.Fatal(err)
	}
	return head.Kind
}

// A newer sample with no older partner is a pair cut in half, so every file in testdata/upgrade is part of one.
func TestEveryUpgradeSampleHasItsPartner(t *testing.T) {
	tos, err := filepath.Glob("testdata/upgrade/*/*/*.v2.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, to := range tos {
		if _, err := os.Stat(strings.TrimSuffix(to, ".v2.json") + ".v1.json"); err != nil {
			t.Errorf("%s has no .v1.json partner", to)
		}
	}
}

// What a step must refuse: an answer that is not of the version it reads.
func TestUpgradeStepsRefuseWhatIsNotTheirs(t *testing.T) {
	refused, err := filepath.Glob("testdata/upgrade/*/*/*.refused.json")
	if err != nil || len(refused) == 0 {
		t.Fatalf("no refused upgrade samples found: %v", err)
	}
	for _, path := range refused {
		t.Run(path, func(t *testing.T) {
			step, _, _, _ := stepOf(t, path)
			if out, err := step(readFile(t, path)); err == nil {
				t.Fatalf("the step took it and gave\n%s", out)
			}
		})
	}
}
