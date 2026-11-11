// Package contracts holds every schema, sample and table of this repository, so a Go app tests its real answers against them.
package contracts

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// FS is every schema (<dir>/<version>/<name>.schema.json), sample (<dir>/<version>/samples/<name>.json) and table (<dir>/<version>/<name>.yaml).
//
//go:embed */v*/*.schema.json */v*/samples/*.json */v*/*.yaml
var FS embed.FS

const schemaSuffix = ".schema.json"

var (
	compileOnce sync.Once
	compiler    *jsonschema.Compiler
	compileErr  error
)

// Validate checks doc, a JSON document, against the schema at path in FS, such as envelope/v1/envelope.schema.json.
func Validate(schema string, doc []byte) error {
	c, err := schemas()
	if err != nil {
		return err
	}
	id, err := schemaID(schema)
	if err != nil {
		return err
	}
	s, err := c.Compile(id)
	if err != nil {
		return fmt.Errorf("%s: %w", schema, err)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("the document is not JSON: %w", err)
	}
	return s.Validate(value)
}

// schemas is one compiler that knows every schema by its $id, so one schema can refer to another.
func schemas() (*jsonschema.Compiler, error) {
	compileOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		c.AssertFormat()
		compileErr = fs.WalkDir(FS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, schemaSuffix) || path.Base(path.Dir(p)) == "samples" {
				return err
			}
			id, doc, err := read(p)
			if err != nil {
				return err
			}
			return c.AddResource(id, doc)
		})
		compiler = c
	})
	return compiler, compileErr
}

func schemaID(schema string) (string, error) {
	id, _, err := read(schema)
	return id, err
}

// read is the schema at p and its $id.
func read(p string) (string, any, error) {
	data, err := FS.ReadFile(p)
	if err != nil {
		return "", nil, fmt.Errorf("no schema %s in contracts: %w", p, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", p, err)
	}
	id, _ := doc.(map[string]any)["$id"].(string)
	if id == "" {
		return "", nil, fmt.Errorf("%s: no $id", p)
	}
	return id, doc, nil
}
