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

// steps is every upgrade step by its folder in testdata/upgrade, with the schemas of the version it reads and the version it makes.
var steps = map[string]struct {
	upgrade  func([]byte) ([]byte, error)
	from, to string
}{
	"envelope/v1-v2": {contracts.UpgradeEnvelope, "envelope/v1/envelope.schema.json", "envelope/v2/envelope.schema.json"},
}

// stepOf is the step of a file in testdata/upgrade/<dir>/<from>-<to>/, or a failure naming the folder with no step.
func stepOf(t *testing.T, path string) (func([]byte) ([]byte, error), string, string) {
	t.Helper()
	folder := filepath.ToSlash(strings.TrimPrefix(filepath.Dir(path), filepath.Join("testdata", "upgrade")+string(filepath.Separator)))
	s, ok := steps[folder]
	if !ok {
		t.Fatalf("no upgrade step for %s", folder)
	}
	return s.upgrade, s.from, s.to
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

// Each pair in testdata/upgrade/<dir>/<from>-<to>/ is an answer of the older version and the answer its step makes of it, each meeting its own version (CT-RULE-3).
func TestUpgradeStepsMatchTheirSamples(t *testing.T) {
	froms, err := filepath.Glob("testdata/upgrade/*/*/*.v1.json")
	if err != nil || len(froms) == 0 {
		t.Fatalf("no upgrade pairs found: %v", err)
	}
	for _, from := range froms {
		t.Run(from, func(t *testing.T) {
			upgrade, fromSchema, toSchema := stepOf(t, from)
			older := readFile(t, from)
			want := readFile(t, strings.TrimSuffix(from, ".v1.json")+".v2.json")
			if err := contracts.Validate(fromSchema, older); err != nil {
				t.Fatalf("the older sample does not meet %s: %v", fromSchema, err)
			}
			if err := contracts.Validate(toSchema, want); err != nil {
				t.Fatalf("the newer sample does not meet %s: %v", toSchema, err)
			}
			got, err := upgrade(older)
			if err != nil {
				t.Fatal(err)
			}
			// A detail is read by people, so <, > and & stay as they are.
			if strings.Contains(string(got), `\u003c`) || strings.Contains(string(got), `\u0026`) {
				t.Fatalf("the upgrade escaped text meant for people:\n%s", got)
			}
			if !reflect.DeepEqual(decode(t, got), decode(t, want)) {
				t.Fatalf("the upgrade gave\n%s\nwant\n%s", got, want)
			}
		})
	}
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
			upgrade, _, _ := stepOf(t, path)
			if out, err := upgrade(readFile(t, path)); err == nil {
				t.Fatalf("the step took it and gave\n%s", out)
			}
		})
	}
}
