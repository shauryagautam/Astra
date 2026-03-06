package commands

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/spf13/cobra"
)

//go:embed stubs/*.tmpl
var stubFS embed.FS

// FieldDef holds parsed field information from the --fields flag.
type FieldDef struct {
	FieldName string // PascalCase Go struct field name
	GoType    string // Go type (string, int, etc.)
	JSONName  string // snake_case JSON tag
	DBName    string // snake_case DB column name
	SQLType   string // SQL column type
}

// TemplateData holds all data passed to code templates.
type TemplateData struct {
	Package   string     // target package name
	Name      string     // PascalCase name
	NameLower string     // lowercase name
	NameSnake string     // snake_case name
	TableName string     // plural snake_case table name
	Fields    []FieldDef // parsed fields from --fields flag
}

var goTypeToSQL = map[string]string{
	"string":    "TEXT",
	"int":       "INTEGER",
	"int64":     "BIGINT",
	"float64":   "DOUBLE PRECISION",
	"bool":      "BOOLEAN",
	"time.Time": "TIMESTAMPTZ",
	"uuid":      "UUID",
}

func GenerateCmd() *cobra.Command {
	var fields string
	var withMigration bool

	cmd := &cobra.Command{
		Use:   "generate [type] [name]",
		Short: "Generate boilerplate code (model, controller, handler, middleware, job, policy, event, migration)",
		Long: `Generate boilerplate code from templates.

Supported types:
  model        Generate a model struct with db.Model embed
  controller   Generate a ResourceController with all CRUD methods
  handler      Generate a standalone handler function
  middleware   Generate a middleware function
  job          Generate a background job struct
  policy       Generate an authorization policy
  event        Generate an event type with listener pattern
  migration    Generate a SQL migration file

Examples:
  astra generate model User --fields "name:string,email:string,age:int"
  astra generate controller Post
  astra generate job SendWelcomeEmail
  astra generate model Product --fields "title:string,price:float64,stock:int" --migration`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			genType := args[0]
			name := args[1]

			parsedFields := parseFields(fields)
			data := buildTemplateData(name, genType, parsedFields)

			switch genType {
			case "model":
				if err := generateFromStub("stubs/model.go.tmpl", "app/models", data); err != nil {
					return err
				}
				if withMigration || fields != "" {
					return generateMigration(data)
				}
			case "controller":
				return generateFromStub("stubs/controller.go.tmpl", "app/http/controllers", data)
			case "handler":
				return generateFromStub("stubs/handler.go.tmpl", "app/http/handlers", data)
			case "middleware":
				return generateFromStub("stubs/middleware.go.tmpl", "app/http/middleware", data)
			case "job":
				return generateFromStub("stubs/job.go.tmpl", "app/jobs", data)
			case "policy":
				return generateFromStub("stubs/policy.go.tmpl", "app/policies", data)
			case "event":
				return generateFromStub("stubs/event.go.tmpl", "app/events", data)
			case "migration":
				return generateMigration(data)
			default:
				return fmt.Errorf("unknown generator type %q.\nSupported: model, controller, handler, middleware, job, policy, event, migration", genType)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&fields, "fields", "", `Comma-separated fields in "name:type" format (e.g. "title:string,price:float64")`)
	cmd.Flags().BoolVar(&withMigration, "migration", false, "Also generate a migration file (for model generator)")

	return cmd
}

func buildTemplateData(name, genType string, fields []FieldDef) TemplateData {
	pascal := toPascalCase(name)
	snake := toSnakeCase(name)
	lower := strings.ToLower(name)

	// Determine package name from genType
	pkg := "models"
	switch genType {
	case "controller":
		pkg = "controllers"
	case "handler":
		pkg = "handlers"
	case "middleware":
		pkg = "middleware"
	case "job":
		pkg = "jobs"
	case "policy":
		pkg = "policies"
	case "event":
		pkg = "events"
	}

	return TemplateData{
		Package:   pkg,
		Name:      pascal,
		NameLower: lower,
		NameSnake: snake,
		TableName: pluralize(snake),
		Fields:    fields,
	}
}

func parseFields(raw string) []FieldDef {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	fields := make([]FieldDef, 0, len(parts))
	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), ":", 2)
		if len(kv) != 2 {
			continue
		}
		name := strings.TrimSpace(kv[0])
		goType := strings.TrimSpace(kv[1])
		sqlType := goTypeToSQL[goType]
		if sqlType == "" {
			sqlType = "TEXT" // fallback
		}
		fields = append(fields, FieldDef{
			FieldName: toPascalCase(name),
			GoType:    goType,
			JSONName:  toSnakeCase(name),
			DBName:    toSnakeCase(name),
			SQLType:   sqlType,
		})
	}
	return fields
}

func generateFromStub(stubPath, outputDir string, data TemplateData) error {
	content, err := stubFS.ReadFile(stubPath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", stubPath, err)
	}

	tmpl, err := template.New(filepath.Base(stubPath)).Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
	}

	filename := fmt.Sprintf("%s/%s.go", outputDir, data.NameSnake)
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	fmt.Printf("  ✓ Created %s\n", filename)
	return nil
}

func generateMigration(data TemplateData) error {
	content, err := stubFS.ReadFile("stubs/migration.sql.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read migration template: %w", err)
	}

	tmpl, err := template.New("migration").Parse(string(content))
	if err != nil {
		return fmt.Errorf("failed to parse migration template: %w", err)
	}

	dir := "database/migrations"
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	filename := fmt.Sprintf("%s/%s_create_%s.sql", dir, timestamp, data.TableName)

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute migration template: %w", err)
	}

	fmt.Printf("  ✓ Created migration %s\n", filename)
	return nil
}

// ─── String Helpers ───────────────────────────────────────────────────

func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteRune(unicode.ToUpper(rune(part[0])))
			result.WriteString(part[1:])
		}
	}
	if result.Len() == 0 {
		return s
	}
	// Ensure first letter is uppercase
	out := result.String()
	if len(out) > 0 && unicode.IsLower(rune(out[0])) {
		return string(unicode.ToUpper(rune(out[0]))) + out[1:]
	}
	return out
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result.WriteByte('_')
		}
		result.WriteRune(unicode.ToLower(r))
	}
	return result.String()
}

func pluralize(s string) string {
	if strings.HasSuffix(s, "s") {
		return s + "es"
	}
	if strings.HasSuffix(s, "y") {
		return s[:len(s)-1] + "ies"
	}
	return s + "s"
}
