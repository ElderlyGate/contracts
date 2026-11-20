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

// step is one version step as the tests run it: up or down, with what it lost (nothing for an up step).
type step func([]byte) ([]byte, []string, error)

func up(f func([]byte) ([]byte, error)) step {
	return func(b []byte) ([]byte, []string, error) {
		out, err := f(b)
		return out, nil, err
	}
}

// steps is every version step by its folder in testdata/upgrade, <dir>/<from>-<to>, with the schemas of the version it reads and the version it makes.
var steps = map[string]struct {
	run      step
	from, to string
}{
	"envelope/v1-v2": {up(contracts.UpgradeEnvelope), "envelope/v1/envelope.schema.json", "envelope/v2/envelope.schema.json"},
	"envelope/v2-v3": {up(contracts.UpgradeEnvelopeToV3), "envelope/v2/envelope.schema.json", "envelope/v3/envelope.schema.json"},
	"envelope/v3-v2": {contracts.DowngradeEnvelopeToV2, "envelope/v3/envelope.schema.json", "envelope/v2/envelope.schema.json"},
	"version/v1-v2":  {up(contracts.UpgradeVersionToV2), "version/v1/version.schema.json", "version/v2/version.schema.json"},
	"version/v2-v1":  {contracts.DowngradeVersionToV1, "version/v2/version.schema.json", "version/v1/version.schema.json"},
}

// stepOf is the step of a file in testdata/upgrade/<dir>/<from>-<to>/, with the versions it reads and makes, or a failure naming the folder with no step.
func stepOf(t *testing.T, path string) (run step, fromSchema, toSchema, from, to string) {
	t.Helper()
	folder := filepath.ToSlash(strings.TrimPrefix(filepath.Dir(path), filepath.Join("testdata", "upgrade")+string(filepath.Separator)))
	s, ok := steps[folder]
	if !ok {
		t.Fatalf("no version step for %s", folder)
	}
	from, to, _ = strings.Cut(filepath.Base(folder), "-")
	return s.run, s.from, s.to, from, to
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

// Each pair in testdata/upgrade/<dir>/<from>-<to>/ is an answer of the version the step reads and the answer the step makes of it, each meeting its own version (CT-RULE-3); <case>.lost.json lists what a down step leaves out, and none means nothing.
func TestVersionStepsMatchTheirSamples(t *testing.T) {
	for folder := range steps {
		t.Run(folder, func(t *testing.T) {
			from, _, _ := strings.Cut(filepath.Base(folder), "-")
			ins, err := filepath.Glob(filepath.Join("testdata", "upgrade", filepath.FromSlash(folder), "*."+from+".json"))
			if err != nil || len(ins) == 0 {
				t.Fatalf("no pairs for %s: %v", folder, err)
			}
			for _, in := range ins {
				t.Run(filepath.Base(in), func(t *testing.T) {
					run, fromSchema, toSchema, fromV, toV := stepOf(t, in)
					case_ := strings.TrimSuffix(in, "."+fromV+".json")
					older := readFile(t, in)
					want := readFile(t, case_+"."+toV+".json")
					if err := contracts.Validate(fromSchema, older); err != nil {
						t.Fatalf("the sample it reads does not meet %s: %v", fromSchema, err)
					}
					if err := contracts.Validate(toSchema, want); err != nil {
						t.Fatalf("the sample it makes does not meet %s: %v", toSchema, err)
					}
					got, lost, err := run(older)
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
					wantLost := []string{}
					if b, err := os.ReadFile(case_ + ".lost.json"); err == nil {
						if err := json.Unmarshal(b, &wantLost); err != nil {
							t.Fatal(err)
						}
					}
					if lost == nil {
						lost = []string{}
					}
					if !reflect.DeepEqual(lost, wantLost) {
						t.Fatalf("the step lost %v, want %v", lost, wantLost)
					}
				})
			}
		})
	}
}

// Every file in testdata/upgrade is part of a pair: what a step makes has what it read beside it, and lost has both.
func TestEveryVersionStepSampleHasItsPartner(t *testing.T) {
	for folder := range steps {
		from, to, _ := strings.Cut(filepath.Base(folder), "-")
		dir := filepath.Join("testdata", "upgrade", filepath.FromSlash(folder))
		files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		for _, f := range files {
			base := filepath.Base(f)
			for _, suffix := range []string{"." + to + ".json", ".lost.json"} {
				if case_, ok := strings.CutSuffix(base, suffix); ok {
					if _, err := os.Stat(filepath.Join(dir, case_+"."+from+".json")); err != nil {
						t.Errorf("%s has no %s.%s.json partner", f, case_, from)
					}
				}
			}
		}
	}
}

// What a step must refuse: an answer that is not of the version it reads.
func TestVersionStepsRefuseWhatIsNotTheirs(t *testing.T) {
	for folder := range steps {
		refused, _ := filepath.Glob(filepath.Join("testdata", "upgrade", filepath.FromSlash(folder), "*.refused.json"))
		if len(refused) == 0 {
			t.Errorf("%s has no refused sample", folder)
		}
		for _, path := range refused {
			t.Run(path, func(t *testing.T) {
				run, _, _, _, _ := stepOf(t, path)
				if out, _, err := run(readFile(t, path)); err == nil {
					t.Fatalf("the step took it and gave\n%s", out)
				}
			})
		}
	}
}
