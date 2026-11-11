package contracts_test

import (
	"encoding/json"
	"fmt"
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

func TestValidateIsSafeFromManyGoroutines(t *testing.T) {
	doc, err := os.ReadFile("envelope/v1/samples/envelope.json")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 8 {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			for range 50 {
				if err := contracts.Validate("envelope/v1/envelope.schema.json", doc); err != nil {
					t.Error(err)
				}
				if err := contracts.Validate("designer/v1/version.schema.json", doc); err == nil {
					t.Error("a greeting was taken as a version")
				}
			}
		})
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

func TestAServerRecordTakesNewSpecFieldsButNoNewHeader(t *testing.T) {
	sample, err := os.ReadFile("mankit/v1/samples/server.json")
	if err != nil {
		t.Fatal(err)
	}
	var rec struct {
		APIVersion string         `json:"apiVersion"`
		Kind       string         `json:"kind"`
		Metadata   map[string]any `json:"metadata"`
		Spec       map[string]any `json:"spec"`
	}
	if err := json.Unmarshal(sample, &rec); err != nil {
		t.Fatal(err)
	}
	rec.Spec["queue"] = "a field a newer app adds"
	newer, _ := json.Marshal(rec)
	if err := contracts.Validate("mankit/v1/server.schema.json", newer); err != nil {
		t.Fatalf("a record from a newer app was refused: %v", err)
	}
	delete(rec.Spec, "queue")
	rec.Metadata["host"] = "a renamed header"
	changed, _ := json.Marshal(rec)
	if err := contracts.Validate("mankit/v1/server.schema.json", changed); err == nil {
		t.Fatal("a record with a new header field was taken")
	}
}

func TestATicketTakesNewSpecFieldsButNoNewHeader(t *testing.T) {
	sample, err := os.ReadFile("mankit/v1/samples/ticket.json")
	if err != nil {
		t.Fatal(err)
	}
	var ticket struct {
		APIVersion string         `json:"apiVersion"`
		Kind       string         `json:"kind"`
		Metadata   map[string]any `json:"metadata"`
		Spec       map[string]any `json:"spec"`
	}
	if err := json.Unmarshal(sample, &ticket); err != nil {
		t.Fatal(err)
	}
	ticket.Spec["priority"] = "a field a newer app adds"
	newer, _ := json.Marshal(ticket)
	if err := contracts.Validate("mankit/v1/ticket.schema.json", newer); err != nil {
		t.Fatalf("a ticket from a newer app was refused: %v", err)
	}
	delete(ticket.Spec, "priority")
	ticket.Metadata["machine"] = ticket.Metadata["host"]
	delete(ticket.Metadata, "host")
	renamed, _ := json.Marshal(ticket)
	if err := contracts.Validate("mankit/v1/ticket.schema.json", renamed); err == nil {
		t.Fatal("a ticket with a renamed header field was taken")
	}
}
