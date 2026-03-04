package codegen

import (
	"fmt"
)

// GenerateClient is the main entry point to parse code and write TS files.
func GenerateClient(sourceDir, outputDir string) error {
	meta, err := Parse(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to parse Go files: %w", err)
	}

	generator := NewGenerator(outputDir)
	if err := generator.Generate(meta); err != nil {
		return fmt.Errorf("failed to generate TypeScript client: %w", err)
	}

	return nil
}
