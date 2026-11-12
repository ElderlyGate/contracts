package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

const schemaSuffix = ".schema.json"

// versionDirs lists every <dir>/<version>/ folder that holds schemas.
func versionDirs(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*/*/*" + schemaSuffix)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var dirs []string
	for _, f := range files {
		if dir := filepath.Dir(f); !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) == 0 {
		t.Fatal("no contract folders found")
	}
	return dirs
}

func readJSON(t *testing.T, path string) any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return doc
}

// compiler holds every schema in the repo, so one schema can refer to another.
func compiler(t *testing.T) *jsonschema.Compiler {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	for _, dir := range versionDirs(t) {
		schemas, _ := filepath.Glob(filepath.Join(dir, "*"+schemaSuffix))
		for _, path := range schemas {
			doc := readJSON(t, path)
			id, _ := doc.(map[string]any)["$id"].(string)
			if id == "" {
				t.Fatalf("%s: no $id", path)
			}
			if err := c.AddResource(id, doc); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
		}
	}
	return c
}

func schemaID(t *testing.T, path string) string {
	t.Helper()
	id, _ := readJSON(t, path).(map[string]any)["$id"].(string)
	return id
}

func TestEverySchemaIsValidAgainstTheDraft(t *testing.T) {
	c := compiler(t)
	for _, dir := range versionDirs(t) {
		schemas, _ := filepath.Glob(filepath.Join(dir, "*"+schemaSuffix))
		for _, path := range schemas {
			t.Run(path, func(t *testing.T) {
				if _, err := c.Compile(schemaID(t, path)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestEverySampleMeetsItsSchema(t *testing.T) {
	c := compiler(t)
	for _, dir := range versionDirs(t) {
		samples, _ := filepath.Glob(filepath.Join(dir, "samples", "*.json"))
		for _, path := range samples {
			t.Run(path, func(t *testing.T) {
				kind := strings.TrimSuffix(filepath.Base(path), ".json")
				schemaPath := filepath.Join(dir, kind+schemaSuffix)
				if _, err := os.Stat(schemaPath); err != nil {
					t.Fatalf("sample has no schema %s", schemaPath)
				}
				schema, err := c.Compile(schemaID(t, schemaPath))
				if err != nil {
					t.Fatal(err)
				}
				if err := schema.Validate(readJSON(t, path)); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestEverySchemaHasASample(t *testing.T) {
	for _, dir := range versionDirs(t) {
		schemas, _ := filepath.Glob(filepath.Join(dir, "*"+schemaSuffix))
		for _, path := range schemas {
			kind := strings.TrimSuffix(filepath.Base(path), schemaSuffix)
			sample := filepath.Join(dir, "samples", kind+".json")
			if _, err := os.Stat(sample); err != nil {
				t.Errorf("%s has no sample %s", path, sample)
			}
		}
	}
}

func TestASampleThatBreaksItsSchemaIsCaught(t *testing.T) {
	c := compiler(t)
	schema, err := c.Compile(schemaID(t, "error/v1/error.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bad map[string]any
	if err := json.Unmarshal([]byte(`{"code":"unsupported_contract","message":"no"}`), &bad); err != nil {
		t.Fatal(err)
	}
	if schema.Validate(bad) == nil {
		t.Fatal("unsupported_contract without supported versions must fail")
	}
}

type asyncMessage struct {
	Name    string `yaml:"name"`
	Payload struct {
		SchemaFormat string `yaml:"schemaFormat"`
		Schema       struct {
			Ref string `yaml:"$ref"`
		} `yaml:"schema"`
	} `yaml:"payload"`
}

type asyncAPI struct {
	AsyncAPI   string `yaml:"asyncapi"`
	Components struct {
		Messages map[string]asyncMessage `yaml:"messages"`
	} `yaml:"components"`
}

const draft2020Format = "application/schema+json;version=draft-2020-12"

// localRefs collects every "#/..." reference in a decoded YAML document.
func localRefs(node any, refs *[]string) {
	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			if ref, ok := child.(string); ok && key == "$ref" && strings.HasPrefix(ref, "#/") {
				*refs = append(*refs, ref)
			}
			localRefs(child, refs)
		}
	case []any:
		for _, child := range v {
			localRefs(child, refs)
		}
	}
}

func resolves(doc any, ref string) bool {
	node := doc
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		m, ok := node.(map[string]any)
		if !ok {
			return false
		}
		if node, ok = m[part]; !ok {
			return false
		}
	}
	return true
}

func TestEveryEventIsListedWithItsSchemaAndSample(t *testing.T) {
	c := compiler(t)
	catalogs, _ := filepath.Glob("*/*/events.asyncapi.yaml")
	if len(catalogs) == 0 {
		t.Fatal("no events catalog found")
	}
	eventSchema, err := c.Compile(schemaID(t, "event/v1/event.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, catalog := range catalogs {
		raw, err := os.ReadFile(catalog)
		if err != nil {
			t.Fatal(err)
		}
		var api asyncAPI
		var doc any
		if err := yaml.Unmarshal(raw, &api); err != nil {
			t.Fatalf("%s: %v", catalog, err)
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: %v", catalog, err)
		}
		if api.AsyncAPI != "3.0.0" {
			t.Errorf("%s: asyncapi is %q, want 3.0.0", catalog, api.AsyncAPI)
		}
		var refs []string
		localRefs(doc, &refs)
		for _, ref := range refs {
			if !resolves(doc, ref) {
				t.Errorf("%s: %s does not resolve", catalog, ref)
			}
		}
		dir := filepath.Dir(catalog)
		listed := map[string]bool{}
		for _, msg := range api.Components.Messages {
			listed[msg.Payload.Schema.Ref] = true
			t.Run(msg.Name, func(t *testing.T) {
				if msg.Payload.SchemaFormat != draft2020Format {
					t.Fatalf("schemaFormat is %q, want %q", msg.Payload.SchemaFormat, draft2020Format)
				}
				schemaPath := filepath.Join(dir, msg.Payload.Schema.Ref)
				if filepath.Base(schemaPath) != msg.Name+schemaSuffix {
					t.Fatalf("payload schema of %s is %s", msg.Name, schemaPath)
				}
				sample := readJSON(t, filepath.Join(dir, "samples", msg.Name+".json"))
				if err := eventSchema.Validate(sample); err != nil {
					t.Fatalf("not an event: %v", err)
				}
				payload, err := c.Compile(schemaID(t, schemaPath))
				if err != nil {
					t.Fatal(err)
				}
				if err := payload.Validate(sample); err != nil {
					t.Fatal(err)
				}
			})
		}
		schemas, _ := filepath.Glob(filepath.Join(dir, "*"+schemaSuffix))
		for _, path := range schemas {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "event/v1/event.schema.json") && !listed[filepath.Base(path)] {
				t.Errorf("%s is an event schema that %s does not list", path, catalog)
			}
		}
	}
}

func TestNoSamplesWithoutSchemas(t *testing.T) {
	known := map[string]bool{}
	for _, dir := range versionDirs(t) {
		known[dir] = true
	}
	samples, _ := filepath.Glob("*/*/samples/*.json")
	for _, path := range samples {
		if dir := filepath.Dir(filepath.Dir(path)); !known[dir] {
			t.Errorf("%s is in a folder with no schemas", path)
		}
	}
}

// Each file in testdata/bad/<dir>/<version>/<kind>/ must fail <dir>/<version>/<kind>.schema.json.
func TestBrokenSamplesAreRefused(t *testing.T) {
	c := compiler(t)
	cases, _ := filepath.Glob("testdata/bad/*/*/*/*.json")
	if len(cases) == 0 {
		t.Fatal("no broken samples found")
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			kindDir := filepath.Dir(path)
			schemaPath := filepath.Join(strings.TrimPrefix(filepath.Dir(kindDir), "testdata/bad/"), filepath.Base(kindDir)+schemaSuffix)
			schema, err := c.Compile(schemaID(t, schemaPath))
			if err != nil {
				t.Fatal(err)
			}
			if schema.Validate(readJSON(t, path)) == nil {
				t.Fatal("a broken sample was accepted")
			}
		})
	}
}

func TestApiVersionFormsListEveryFormOnce(t *testing.T) {
	c := compiler(t)
	schema, err := c.Compile(schemaID(t, "apiversion/v1/forms.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Forms []struct {
			New     string   `yaml:"new"`
			Old     []string `yaml:"old"`
			App     string   `yaml:"app"`
			Upgrade string   `yaml:"upgrade"`
		} `yaml:"forms"`
	}
	raw, err := os.ReadFile("apiversion/v1/forms.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatal(err)
	}
	envelope := readJSON(t, "envelope/v1/envelope.schema.json").(map[string]any)
	pattern := envelope["properties"].(map[string]any)["apiVersion"].(map[string]any)["pattern"].(string)
	newForm := regexp.MustCompile(pattern)
	current := map[string]bool{}
	oldIn := map[string]string{}
	for _, row := range table.Forms {
		if !newForm.MatchString(row.New) {
			t.Errorf("new form %q is not an envelope apiVersion", row.New)
		}
		if current[row.New] {
			t.Errorf("form %s is listed twice", row.New)
		}
		current[row.New] = true
		if len(row.Old) == 0 {
			continue
		}
		if want := "hubman migrate " + row.App + " <folder>"; row.Upgrade != want {
			t.Errorf("%s: upgrade is %q, want %q", row.New, row.Upgrade, want)
		}
		for _, old := range row.Old {
			if other, seen := oldIn[old]; seen {
				t.Errorf("%s is an old form of both %s and %s", old, other, row.New)
			}
			oldIn[old] = row.New
		}
	}
}
