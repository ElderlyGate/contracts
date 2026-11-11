package contracts_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ElderlyGate/contracts"
)

func TestValidateTakesASample(t *testing.T) {
	samples, err := filepath.Glob("*/*/samples/*.json")
	if err != nil || len(samples) == 0 {
		t.Fatalf("no samples: %v", err)
	}
	for _, sample := range samples {
		name := strings.TrimSuffix(filepath.Base(sample), ".json")
		schema := filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Dir(sample)), name+".schema.json"))
		doc, err := os.ReadFile(sample)
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.Validate(schema, doc); err != nil {
			t.Errorf("%s: %v", sample, err)
		}
	}
}

func TestValidateRefusesABrokenDocument(t *testing.T) {
	doc, err := os.ReadFile("testdata/bad/envelope/v1/envelope/missing-ran.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := contracts.Validate("envelope/v1/envelope.schema.json", doc); err == nil {
		t.Fatal("a document without ran was taken")
	}
}

func TestValidateFollowsAReferenceToAnotherSchema(t *testing.T) {
	doc := []byte(`{"apiVersion":"elderlygate/example/app/v1","kind":"Greeting","ran":{"name":"example","app":"example","version":"1.0.0","contract":"v1"},"data":null,` +
		`"errors":[{"apiVersion":"elderlygate/example/app/v1","kind":"Error","code":"unsupported_contract","message":"no v3"}]}`)
	if err := contracts.Validate("envelope/v1/envelope.schema.json", doc); err == nil {
		t.Fatal("an unsupported_contract error without its supported versions was taken")
	}
}

func TestValidateNamesAMissingSchema(t *testing.T) {
	err := contracts.Validate("nothing/v1/nothing.schema.json", []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), "nothing/v1/nothing.schema.json") {
		t.Fatalf("got %v, want an error naming the schema", err)
	}
}

func TestValidateRefusesADocumentThatIsNotJSON(t *testing.T) {
	if err := contracts.Validate("envelope/v1/envelope.schema.json", []byte("not json")); err == nil {
		t.Fatal("a document that is not JSON was taken")
	}
}
