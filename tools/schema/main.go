//go:build ignore

// Schema generates the JSON Schema for the afon configuration file and writes
// it to schemas/afon.json. Run it from the module root with:
//
//	go run ./tools/schema/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/n3tuk/afon/internal/config"
)

// Schema represents a JSON Schema object. Fields are declared in the order
// they appear in the marshalled JSON output.
type Schema struct {
	SchemaURL            string             `json:"$schema,omitempty"`
	ID                   string             `json:"$id,omitempty"`
	Title                string             `json:"title,omitempty"`
	Description          string             `json:"description,omitempty"`
	Type                 string             `json:"type,omitempty"`
	AdditionalProperties *bool              `json:"additionalProperties,omitempty"`
	Required             []string           `json:"required,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	MinLength            *int               `json:"minLength,omitempty"`
}

// fieldMeta holds schema annotations for a field identified by its full
// dot-separated path (e.g. "template.source"). The root Config path is ".".
type fieldMeta struct {
	description string
	required    bool
	minLength   *int
}

// metadata maps every field path to its schema annotations. Update this map
// whenever the Config or Template structs change.
var metadata = map[string]fieldMeta{ //nolint:gochecknoglobals // generator-only package-level map
	".": {
		description: "Configuration for afon, the upstream template repository applicator.",
	},
	"template": {
		description: "Settings for the upstream template repository.",
		required:    true,
	},
	"template.source": {
		description: "Local filesystem path or remote Git URL pointing to the upstream template repository.",
		required:    true,
		minLength:   intPtr(1),
	},
	"template.ref": {
		description: "Branch, tag, or commit SHA to check out when fetching a remote template repository. Ignored for local paths.",
	},
	"template.path": {
		description: "Optional path within the template repository that holds the template files. When set, only files within this path are processed and the path prefix is stripped from all output paths.",
	},
	"template.token": {
		description: "Personal access token for authenticating against private remote template repositories. If empty, the GITHUB_TOKEN environment variable is used as a fallback.",
	},
	"variables": {
		description: "Free-form map of values passed to the template engine as the rendering context. Template authors access these as {{ .key }}.",
	},
}

func main() {
	schema := buildSchema(reflect.TypeOf(config.Config{}), ".")
	schema.SchemaURL = "https://json-schema.org/draft/2020-12/schema"
	schema.ID = "https://raw.githubusercontent.com/n3tuk/afon/main/schemas/afon.json"
	schema.Title = "afon"

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshalling schema: %v\n", err)
		os.Exit(1)
	}

	output := filepath.Join("schemas", "afon.json")

	if err = os.MkdirAll(filepath.Dir(output), 0o750); err != nil {
		fmt.Fprintf(os.Stderr, "error creating directory: %v\n", err)
		os.Exit(1)
	}

	if err = os.WriteFile(output, append(data, '\n'), 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "error writing schema: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("written %s\n", output)
}

// buildSchema recursively generates a JSON Schema for the given reflected type
// at the given dot-separated metadata path.
func buildSchema(t reflect.Type, path string) *Schema {
	meta := metadata[path]

	switch t.Kind() {
	case reflect.Struct:
		return buildObjectSchema(t, path, meta)
	case reflect.Map:
		return buildMapSchema(meta)
	default:
		return buildStringSchema(meta)
	}
}

// buildObjectSchema generates a JSON Schema for a struct type.
func buildObjectSchema(t reflect.Type, path string, meta fieldMeta) *Schema {
	schema := &Schema{
		Description:          meta.description,
		Type:                 "object",
		AdditionalProperties: boolPtr(false),
		Properties:           make(map[string]*Schema),
	}

	for i := range t.NumField() {
		field := t.Field(i)

		name := field.Tag.Get("mapstructure")
		if name == "" || name == "-" {
			continue
		}

		childPath := name
		if path != "." {
			childPath = path + "." + name
		}

		schema.Properties[name] = buildSchema(field.Type, childPath)

		if metadata[childPath].required {
			schema.Required = append(schema.Required, name)
		}
	}

	return schema
}

// buildMapSchema generates a JSON Schema for a map[string]any field (open
// object — any additional properties are permitted).
func buildMapSchema(meta fieldMeta) *Schema {
	return &Schema{
		Description:          meta.description,
		Type:                 "object",
		AdditionalProperties: boolPtr(true),
	}
}

// buildStringSchema generates a JSON Schema for a string field.
func buildStringSchema(meta fieldMeta) *Schema {
	return &Schema{
		Description: meta.description,
		Type:        "string",
		MinLength:   meta.minLength,
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(n int) *int    { return &n }
