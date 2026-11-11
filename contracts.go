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
	compiled    map[string]*jsonschema.Schema
	compileErr  error
)

// Validate checks doc, a JSON document, against the schema at path in FS, such as envelope/v1/envelope.schema.json; many goroutines may call it at once.
func Validate(schema string, doc []byte) error {
	all, err := schemas()
	if err != nil {
		return err
	}
	s, ok := all[schema]
	if !ok {
		return fmt.Errorf("no schema %s in contracts", schema)
	}
	value, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("the document is not JSON: %w", err)
	}
	return s.Validate(value)
}

// schemas compiles every schema once, by its path: the compiler is not safe for many goroutines, a compiled schema is.
func schemas() (map[string]*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.DefaultDraft(jsonschema.Draft2020)
		c.AssertFormat()
		ids := map[string]string{}
		compileErr = fs.WalkDir(FS, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, schemaSuffix) || path.Base(path.Dir(p)) == "samples" {
				return err
			}
			id, doc, err := read(p)
			if err != nil {
				return err
			}
			ids[p] = id
			return c.AddResource(id, doc)
		})
		if compileErr != nil {
			return
		}
		all := map[string]*jsonschema.Schema{}
		for p, id := range ids {
			s, err := c.Compile(id)
			if err != nil {
				compileErr = fmt.Errorf("%s: %w", p, err)
				return
			}
			all[p] = s
		}
		compiled = all
	})
	return compiled, compileErr
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
