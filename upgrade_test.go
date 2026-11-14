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

// Each pair in testdata/upgrade/envelope/v1-v2/ is a v1 answer and the v2 answer UpgradeEnvelope makes of it, each meeting its own version (CT-RULE-3).
func TestUpgradeStepsMatchTheirSamples(t *testing.T) {
	froms, err := filepath.Glob("testdata/upgrade/envelope/v1-v2/*.v1.json")
	if err != nil || len(froms) == 0 {
		t.Fatalf("no upgrade pairs found: %v", err)
	}
	for _, from := range froms {
		t.Run(from, func(t *testing.T) {
			v1, err := os.ReadFile(from)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(strings.TrimSuffix(from, ".v1.json") + ".v2.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := contracts.Validate("envelope/v1/envelope.schema.json", v1); err != nil {
				t.Fatalf("the v1 sample is not envelope/v1: %v", err)
			}
			if err := contracts.Validate("envelope/v2/envelope.schema.json", want); err != nil {
				t.Fatalf("the v2 sample is not envelope/v2: %v", err)
			}
			got, err := contracts.UpgradeEnvelope(v1)
			if err != nil {
				t.Fatal(err)
			}
			var g, w any
			_ = json.Unmarshal(got, &g)
			_ = json.Unmarshal(want, &w)
			if !reflect.DeepEqual(g, w) {
				t.Fatalf("the upgrade gave\n%s\nwant\n%s", got, want)
			}
		})
	}
}
