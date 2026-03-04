package graphql

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/graph-gophers/graphql-go"
)

// LoadSchema reads all .graphql files in a directory and parses them into a Schema.
func LoadSchema(dir string, resolver interface{}, opts ...graphql.SchemaOpt) (*graphql.Schema, error) {
	var sb strings.Builder

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".graphql") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", path, err)
		}

		sb.Write(content)
		sb.WriteString("\n")
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load schema files: %w", err)
	}

	schemaStr := sb.String()
	if schemaStr == "" {
		return nil, fmt.Errorf("no .graphql files found in %s", dir)
	}

	return graphql.ParseSchema(schemaStr, resolver, opts...)
}
