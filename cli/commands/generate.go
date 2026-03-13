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
	cmd := &cobra.Command{
		Use:   "generate [type] [name]",
		Short: "Generate boilerplate code",
		Long:  `Generate boilerplate code from templates. Supported types: model, controller, handler, middleware, job, policy, event, listener, provider, migration, mailer, rule, seeder, factory`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerator(args[0], args[1], cmd)
		},
	}

	cmd.PersistentFlags().String("fields", "", `Comma-separated fields in "name:type" format`)
	cmd.PersistentFlags().Bool("migration", false, "Also generate a migration file")

	return cmd
}

func MakeCmd() *cobra.Command {
	makeCmd := &cobra.Command{
		Use:   "make",
		Short: "Scaffold framework components",
	}

	types := []string{"model", "controller", "handler", "middleware", "job", "policy", "event", "listener", "provider", "migration", "mailer", "rule", "seeder", "factory"}
	for _, t := range types {
		makeCmd.AddCommand(createSubcommand(t))
	}

	return makeCmd
}

// AddMakeAliases registers make:* style commands directly to the root.
func AddMakeAliases(rootCmd *cobra.Command) {
	types := []string{"model", "controller", "handler", "middleware", "job", "policy", "event", "listener", "provider", "migration", "mailer", "rule", "seeder", "factory"}
	for _, t := range types {
		cmd := createSubcommand(t)
		cmd.Use = "make:" + t + " [name]"
		cmd.Hidden = true // Keep main help clean, but allow usage
		rootCmd.AddCommand(cmd)
	}
}

func createSubcommand(genType string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   genType + " [name]",
		Short: fmt.Sprintf("Generate a %s", genType),
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerator(genType, args[0], cmd)
		},
	}
	cmd.Flags().String("fields", "", `Comma-separated fields in "name:type" format`)
	cmd.Flags().Bool("migration", false, "Also generate a migration file")
	return cmd
}

func runGenerator(genType, name string, cmd *cobra.Command) error {
	fields, _ := cmd.Flags().GetString("fields")
	if fields == "" && cmd.Parent() != nil {
		fields, _ = cmd.Parent().PersistentFlags().GetString("fields")
	}
	withMigration, _ := cmd.Flags().GetBool("migration")
	if !withMigration && cmd.Parent() != nil {
		withMigration, _ = cmd.Parent().PersistentFlags().GetBool("migration")
	}

	if name == "" {
		return fmt.Errorf("name is required")
	}

	// Simple validation: ensure name starts with a letter and contains only alphanumeric
	for i, r := range name {
		if i == 0 && !unicode.IsLetter(r) {
			return fmt.Errorf("invalid name: must start with a letter")
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return fmt.Errorf("invalid name: contains invalid characters")
		}
	}

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
	case "listener":
		return generateFromStub("stubs/listener.go.tmpl", "app/listeners", data)
	case "provider":
		return generateFromStub("stubs/provider.go.tmpl", "app/providers", data)
	case "migration":
		return generateMigration(data)
	case "mailer":
		return generateFromStub("stubs/mailer.go.tmpl", "app/mailers", data)
	case "rule":
		return generateFromStub("stubs/rule.go.tmpl", "app/rules", data)
	case "seeder":
		return generateFromStub("stubs/seeder.go.tmpl", "database/seeders", data)
	case "factory":
		return generateFromStub("stubs/factory.go.tmpl", "database/factories", data)
	default:
		return fmt.Errorf("unknown generator type %q", genType)
	}
	return nil
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
	case "listener":
		pkg = "listeners"
	case "provider":
		pkg = "providers"
	case "mailer":
		pkg = "mailers"
	case "rule":
		pkg = "rules"
	case "seeder":
		pkg = "seeders"
	case "factory":
		pkg = "factories"
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

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
	}

	filename := filepath.Join(outputDir, data.NameSnake+".go")
	cleanPath := filepath.Clean(filename)
	if !filepath.IsLocal(cleanPath) || strings.Contains(data.NameSnake, "..") || strings.ContainsAny(data.NameSnake, "/\\") {
		return fmt.Errorf("invalid filename: path traversal detected")
	}
	file, err := os.Create(cleanPath) // #nosec G304 -- path validated above
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filename, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Ignore close error
		}
	}()

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
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	filename := filepath.Join(dir, timestamp+"_create_"+data.TableName+".sql")
	if !filepath.IsLocal(filename) || strings.Contains(data.TableName, "..") {
		return fmt.Errorf("invalid migration filename: path traversal detected")
	}

	file, err := os.Create(filename) // #nosec G304 -- path validated above
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Ignore close error
		}
	}()

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
			// Add underscore if previous char was lowercase
			prev := rune(s[i-1])
			if !unicode.IsUpper(prev) && prev != '_' {
				result.WriteByte('_')
			}
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
