package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(makeControllerCmd)
	rootCmd.AddCommand(makeModelCmd)
	rootCmd.AddCommand(makeMigrationCmd)
	rootCmd.AddCommand(makeMiddlewareCmd)
	rootCmd.AddCommand(makeProviderCmd)
}

// ══════════════════════════════════════════════════════════════════════
// make:controller
// ══════════════════════════════════════════════════════════════════════

var makeControllerCmd = &cobra.Command{
	Use:   "make:controller [name]",
	Short: "Create a new controller",
	Long:  `Scaffolds a new HTTP controller in app/Controllers/Http/`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		resource, _ := cmd.Flags().GetBool("resource")
		return scaffoldController(name, resource)
	},
}

func init() {
	makeControllerCmd.Flags().BoolP("resource", "r", false, "Generate a resource controller with CRUD methods")
}

func scaffoldController(name string, resource bool) error {
	if !strings.HasSuffix(name, "Controller") {
		name += "Controller"
	}

	dir := filepath.Join("app", "Controllers", "Http")
	filename := toSnakeCase(name) + ".go"
	path := filepath.Join(dir, filename)

	tmpl := controllerTemplate
	if resource {
		tmpl = resourceControllerTemplate
	}

	return writeTemplate(path, tmpl, map[string]string{
		"Name":    name,
		"Package": "http",
	})
}

const controllerTemplate = `package {{.Package}}

import "github.com/shaurya/astra/contracts"

// {{.Name}} handles HTTP requests.
type {{.Name}} struct{}

// New{{.Name}} creates a new {{.Name}}.
func New{{.Name}}() *{{.Name}} {
	return &{{.Name}}{}
}

// Handle is the default handler method.
func (c *{{.Name}}) Handle(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]any{
		"message": "Hello from {{.Name}}",
	})
}
`

const resourceControllerTemplate = `package {{.Package}}

import "github.com/shaurya/astra/contracts"

// {{.Name}} handles RESTful resource requests.
type {{.Name}} struct{}

// New{{.Name}} creates a new {{.Name}}.
func New{{.Name}}() *{{.Name}} {
	return &{{.Name}}{}
}

// Index lists all resources.
// GET /resource
func (c *{{.Name}}) Index(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]any{
		"data": []any{},
	})
}

// Store creates a new resource.
// POST /resource
func (c *{{.Name}}) Store(ctx contracts.HttpContextContract) error {
	// body := ctx.Request().All()
	return ctx.Response().Status(201).Json(map[string]any{
		"message": "Created successfully",
	})
}

// Show displays a single resource.
// GET /resource/:id
func (c *{{.Name}}) Show(ctx contracts.HttpContextContract) error {
	id := ctx.Param("id")
	return ctx.Response().Json(map[string]any{
		"id": id,
	})
}

// Update modifies an existing resource.
// PUT/PATCH /resource/:id
func (c *{{.Name}}) Update(ctx contracts.HttpContextContract) error {
	id := ctx.Param("id")
	return ctx.Response().Json(map[string]any{
		"id":      id,
		"message": "Updated successfully",
	})
}

// Destroy removes a resource.
// DELETE /resource/:id
func (c *{{.Name}}) Destroy(ctx contracts.HttpContextContract) error {
	_ = ctx.Param("id")
	return ctx.Response().NoContent()
}
`

// ══════════════════════════════════════════════════════════════════════
// make:model
// ══════════════════════════════════════════════════════════════════════

var makeModelCmd = &cobra.Command{
	Use:   "make:model [name]",
	Short: "Create a new model",
	Long:  `Scaffolds a new Lucid model in app/Models/`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		migration, _ := cmd.Flags().GetBool("migration")
		if err := scaffoldModel(name); err != nil {
			return err
		}
		if migration {
			tableName := toSnakeCase(name) + "s"
			return scaffoldMigration("create_"+tableName+"_table", tableName)
		}
		return nil
	},
}

func init() {
	makeModelCmd.Flags().BoolP("migration", "m", false, "Also create a migration for this model")
}

func scaffoldModel(name string) error {
	dir := filepath.Join("app", "Models")
	filename := toSnakeCase(name) + ".go"
	path := filepath.Join(dir, filename)

	tableName := toSnakeCase(name) + "s"

	return writeTemplate(path, modelTemplate, map[string]string{
		"Name":      name,
		"TableName": tableName,
	})
}

const modelTemplate = `package models

import "github.com/shaurya/astra/app/Models"

// {{.Name}} represents the {{.TableName}} table.
type {{.Name}} struct {
	models.BaseModel
	// Add your fields here
	// Name  string ` + "`" + `json:"name" gorm:"not null"` + "`" + `
	// Email string ` + "`" + `json:"email" gorm:"uniqueIndex;not null"` + "`" + `
}

// TableName returns the database table name.
func ({{.Name}}) TableName() string {
	return "{{.TableName}}"
}
`

// ══════════════════════════════════════════════════════════════════════
// make:migration
// ══════════════════════════════════════════════════════════════════════

var makeMigrationCmd = &cobra.Command{
	Use:   "make:migration [name]",
	Short: "Create a new migration",
	Long:  `Scaffolds a new database migration in database/migrations/`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		return scaffoldMigration(name, "")
	},
}

func scaffoldMigration(name string, tableName string) error {
	dir := filepath.Join("database", "migrations")
	timestamp := time.Now().Format("20060102150405")
	filename := timestamp + "_" + toSnakeCase(name) + ".go"
	path := filepath.Join(dir, filename)

	if tableName == "" {
		tableName = toSnakeCase(name)
	}

	return writeTemplate(path, migrationTemplate, map[string]string{
		"Name":      toPascalCase(name),
		"TableName": tableName,
		"FullName":  timestamp + "_" + toSnakeCase(name),
	})
}

const migrationTemplate = `package migrations

import (
	"github.com/shaurya/astra/database"
	"gorm.io/gorm"
)

// {{.Name}} migration.
var {{.Name}} = database.Migration{
	Name: "{{.FullName}}",
	Up: func(db *gorm.DB) error {
		schema := database.NewSchema(db)
		return schema.Raw(` + "`" + `
			CREATE TABLE IF NOT EXISTS {{.TableName}} (
				id BIGSERIAL PRIMARY KEY,
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				deleted_at TIMESTAMPTZ
			)
		` + "`" + `)
	},
	Down: func(db *gorm.DB) error {
		schema := database.NewSchema(db)
		return schema.DropTableIfExists("{{.TableName}}")
	},
}
`

// ══════════════════════════════════════════════════════════════════════
// make:middleware
// ══════════════════════════════════════════════════════════════════════

var makeMiddlewareCmd = &cobra.Command{
	Use:   "make:middleware [name]",
	Short: "Create a new middleware",
	Long:  `Scaffolds a new middleware in app/Middleware/`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return scaffoldMiddleware(args[0])
	},
}

func scaffoldMiddleware(name string) error {
	if !strings.HasSuffix(name, "Middleware") {
		name += "Middleware"
	}

	dir := filepath.Join("app", "Middleware")
	filename := toSnakeCase(name) + ".go"
	path := filepath.Join(dir, filename)

	return writeTemplate(path, middlewareTemplate, map[string]string{
		"Name":         name,
		"FunctionName": name,
	})
}

const middlewareTemplate = `package middleware

import "github.com/shaurya/astra/contracts"

// {{.Name}} is a custom middleware.
func {{.FunctionName}}() contracts.MiddlewareFunc {
	return func(ctx contracts.HttpContextContract, next func() error) error {
		// Add your middleware logic here
		// Example: check auth, log, modify headers, etc.

		// Call the next middleware/handler
		return next()
	}
}
`

// ══════════════════════════════════════════════════════════════════════
// make:provider
// ══════════════════════════════════════════════════════════════════════

var makeProviderCmd = &cobra.Command{
	Use:   "make:provider [name]",
	Short: "Create a new service provider",
	Long:  `Scaffolds a new service provider in providers/`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return scaffoldProvider(args[0])
	},
}

func scaffoldProvider(name string) error {
	if !strings.HasSuffix(name, "Provider") {
		name += "Provider"
	}

	dir := "providers"
	filename := toSnakeCase(name) + ".go"
	path := filepath.Join(dir, filename)

	return writeTemplate(path, providerTemplate, map[string]string{
		"Name": name,
	})
}

const providerTemplate = `package providers

import "github.com/shaurya/astra/contracts"

// {{.Name}} is a custom service provider.
type {{.Name}} struct {
	BaseProvider
}

// New{{.Name}} creates a new {{.Name}}.
func New{{.Name}}(app contracts.ApplicationContract) *{{.Name}} {
	return &{{.Name}}{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds services into the container.
func (p *{{.Name}}) Register() error {
	// p.App.Singleton("Namespace/Binding", func(c contracts.ContainerContract) (any, error) {
	//     return NewService(), nil
	// })
	return nil
}

// Boot is called after all providers have been registered.
func (p *{{.Name}}) Boot() error {
	return nil
}
`

// ══════════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════════

func writeTemplate(path string, tmplStr string, data map[string]string) error {
	// Create directory
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s", path)
	}

	// Parse and execute template
	tmpl, err := template.New("scaffold").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("template execute error: %w", err)
	}

	fmt.Printf("✅ Created: %s\n", path)
	return nil
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(r + 32) // lowercase
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteByte(part[0] - 32) // uppercase first letter
			result.WriteString(part[1:])
		}
	}
	return result.String()
}
