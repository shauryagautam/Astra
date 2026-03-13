package codegen

import (
	"fmt"
	"github.com/astraframework/astra/json"
	"os"
	"path/filepath"
	"strings"
)

// GenerateOpenAPI creates an OpenAPI 3.1 specification.
func (g *Generator) GenerateOpenAPI(meta *Metadata) error {
	spec := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Astra API",
			"version": "1.0.0",
		},
		"paths": make(map[string]any),
		"components": map[string]any{
			"schemas": make(map[string]any),
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
		"security": []any{
			map[string]any{"bearerAuth": []string{}},
		},
	}

	paths := spec["paths"].(map[string]any)
	components := spec["components"].(map[string]any)
	schemas := make(map[string]any)
	components["schemas"] = schemas

	// 1. Generate schemas from structs
	for _, s := range meta.Structs {
		schema := map[string]any{
			"type":        "object",
			"description": s.Description,
			"properties":  make(map[string]any),
		}
		props := schema["properties"].(map[string]any)
		required := []string{}

		for _, f := range s.Fields {
			jsonName := f.Name
			if strings.Contains(f.Tag, "json:") {
				parts := strings.Split(f.Tag, "\"")
				if len(parts) >= 3 {
					jsonName = strings.Split(parts[1], ",")[0]
				}
			}
			if jsonName == "-" {
				continue
			}

			prop := map[string]any{
				"type": goTypeToOpenAPIType(f.Type),
			}
			if strings.Contains(f.Tag, "validate:\"required\"") {
				required = append(required, jsonName)
			}
			props[jsonName] = prop
		}
		if len(required) > 0 {
			schema["required"] = required
		}
		schemas[s.Name] = schema
	}

	// 2. Generate paths from routes
	for _, r := range meta.Routes {
		if r.Method == "WS" {
			continue // Skip WebSockets in standard OpenAPI for now
		}

		pathItem, ok := paths[r.Path].(map[string]any)
		if !ok {
			pathItem = make(map[string]any)
			paths[r.Path] = pathItem
		}

		op := map[string]any{
			"operationId": r.Name,
			"summary":     r.Description,
			"description": r.Description,
			"responses": map[string]any{
				"200": map[string]any{
					"description": "Successful response",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{
								"$ref": fmt.Sprintf("#/components/schemas/%s", r.Output),
							},
						},
					},
				},
			},
		}

		if r.Input != "" && r.Input != "any" && r.Input != "unknown" {
			op["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"$ref": fmt.Sprintf("#/components/schemas/%s", r.Input),
						},
					},
				},
			}
		}

		pathItem[strings.ToLower(r.Method)] = op
	}

	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(g.OutputDir, "openapi.json"), b, 0600)
}

func goTypeToOpenAPIType(goType string) string {
	if strings.HasPrefix(goType, "[]") {
		return "array"
	}
	if strings.Contains(goType, "map[") {
		return "object"
	}

	switch goType {
	case "string", "time.Time", "uuid.UUID":
		return "string"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	default:
		return "object"
	}
}
