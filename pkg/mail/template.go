package mail

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
)

// TemplateRenderer handles HTML email templates.
type TemplateRenderer struct {
	fs fs.FS
}

// NewTemplateRenderer creates a new TemplateRenderer.
func NewTemplateRenderer(fileSystem fs.FS) *TemplateRenderer {
	return &TemplateRenderer{fs: fileSystem}
}

// Render renders a template with the given data.
func (r *TemplateRenderer) Render(name string, data any) (string, error) {
	tmpl, err := template.ParseFS(r.fs, name)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}
