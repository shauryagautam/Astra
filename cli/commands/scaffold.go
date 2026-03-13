package commands

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
)

//go:embed stubs/scaffold/*.tmpl
var scaffoldFS embed.FS

// ScaffoldData holds data for scaffold templates
type ScaffoldData struct {
	AppName     string
	PackageName string
	AuthName    string
	ModelName   string
	ModelLower  string
	ModelSnake  string
	ModelPlural string
	Fields      []FieldDef
}

func ScaffoldCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scaffold [type] [name]",
		Short: "Generate complete ready-to-go scaffolds",
		Long: `Generate complete scaffolds with UI and backend.
Supported types: auth, crud`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScaffold(args[0], args[1], cmd)
		},
	}

	// Common flags for all scaffold types
	cmd.Flags().String("fields", "", `Comma-separated fields in "name:type" format (for crud)`)
	cmd.Flags().String("table", "", `Custom table name (for crud)`)
	cmd.Flags().Bool("api-only", false, `Generate API only (no UI)`)

	return cmd
}

func AddScaffoldAliases(rootCmd *cobra.Command) {
	types := []string{"auth", "crud"}
	for _, t := range types {
		cmd := &cobra.Command{
			Use:   "scaffold:" + t + " [name]",
			Short: fmt.Sprintf("Generate %s scaffold", t),
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return runScaffold(t, args[0], cmd)
			},
			Hidden: true, // Keep main help clean
		}

		// Copy flags from main scaffold command
		cmd.Flags().String("fields", "", `Comma-separated fields in "name:type" format (for crud)`)
		cmd.Flags().String("table", "", `Custom table name (for crud)`)
		cmd.Flags().Bool("api-only", false, `Generate API only (no UI)`)

		rootCmd.AddCommand(cmd)
	}
}

func runScaffold(scaffoldType, name string, cmd *cobra.Command) error {
	switch scaffoldType {
	case "auth":
		return generateAuthScaffold(name, cmd)
	case "crud":
		return generateCRUDScaffold(name, cmd)
	default:
		return fmt.Errorf("unsupported scaffold type: %s. Supported: auth, crud", scaffoldType)
	}
}

func generateAuthScaffold(name string, cmd *cobra.Command) error {
	fmt.Printf("🔐 Generating Auth scaffold: %s\n", name)

	data := ScaffoldData{
		AppName:     name,
		PackageName: strings.ToLower(name),
		AuthName:    name,
		ModelName:   "User",
		ModelLower:  "user",
		ModelSnake:  "users",
		ModelPlural: "Users",
	}

	// Generate auth files
	files := map[string]string{
		"app/http/controllers/auth_controller.go.tmpl": "http/controllers/auth_controller.go",
		"app/http/middleware/auth_middleware.go.tmpl":  "http/middleware/auth_middleware.go",
		"app/models/user.go.tmpl":                      "models/user.go",
		"app/policies/user_policy.go.tmpl":             "policies/user_policy.go",
		"app/migrations/create_users_table.go.tmpl":    "migrations/create_users_table.go",
		"config/auth.go.tmpl":                          "config/auth.go",
	}

	if apiOnly, _ := cmd.Flags().GetBool("api-only"); !apiOnly {
		files["app/views/auth/login.go.tmpl"] = "views/auth/login.go"
		files["app/views/auth/register.go.tmpl"] = "views/auth/register.go"
		files["app/views/auth/dashboard.go.tmpl"] = "views/auth/dashboard.go"
		files["app/static/auth.css.tmpl"] = "static/auth.css"
		files["app/static/auth.js.tmpl"] = "static/auth.js"
	}

	return generateScaffoldFiles(data, files)
}

func generateCRUDScaffold(name string, cmd *cobra.Command) error {
	fmt.Printf("🔧 Generating CRUD scaffold: %s\n", name)

	fieldsStr, _ := cmd.Flags().GetString("fields")
	tableName, _ := cmd.Flags().GetString("table")
	apiOnly, _ := cmd.Flags().GetBool("api-only")

	if fieldsStr == "" {
		return fmt.Errorf("CRUD scaffold requires --fields flag (e.g., --fields=\"name:string,email:string,age:int\")")
	}

	fields := parseFields(fieldsStr)

	if tableName == "" {
		tableName = toSnakeCase(name) + "s"
	}

	data := ScaffoldData{
		AppName:     name,
		PackageName: strings.ToLower(name),
		ModelName:   toPascalCase(name),
		ModelLower:  strings.ToLower(name),
		ModelSnake:  toSnakeCase(name),
		ModelPlural: tableName,
		Fields:      fields,
	}

	// Generate CRUD files
	files := map[string]string{
		"app/http/controllers/" + strings.ToLower(name) + "_controller.go.tmpl": "http/controllers/" + strings.ToLower(name) + "_controller.go",
		"app/models/" + strings.ToLower(name) + ".go.tmpl":                      "models/" + strings.ToLower(name) + ".go",
		"app/policies/" + strings.ToLower(name) + "_policy.go.tmpl":             "policies/" + strings.ToLower(name) + "_policy.go",
		"app/migrations/create_" + tableName + "_table.go.tmpl":                 "migrations/create_" + tableName + "_table.go",
		"app/requests/" + strings.ToLower(name) + "_request.go.tmpl":            "requests/" + strings.ToLower(name) + "_request.go",
		"app/resources/" + strings.ToLower(name) + "_resource.go.tmpl":          "resources/" + strings.ToLower(name) + "_resource.go",
	}

	if apiOnly, _ := cmd.Flags().GetBool("api-only"); !apiOnly {
		files["app/views/"+strings.ToLower(name)+"/index.go.tmpl"] = "views/" + strings.ToLower(name) + "/index.go"
		files["app/views/"+strings.ToLower(name)+"/show.go.tmpl"] = "views/" + strings.ToLower(name) + "/show.go"
		files["app/views/"+strings.ToLower(name)+"/form.go.tmpl"] = "views/" + strings.ToLower(name) + "/form.go"
		files["app/static/"+strings.ToLower(name)+".css.tmpl"] = "static/" + strings.ToLower(name) + ".css"
	}

	return generateScaffoldFiles(data, files)
}

func generateScaffoldFiles(data ScaffoldData, files map[string]string) error {
	for templatePath, outputPath := range files {
		content, err := scaffoldFS.ReadFile(templatePath)
		if err != nil {
			fmt.Printf("⚠️  Template not found: %s (skipping)\n", templatePath)
			continue
		}

		tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(content))
		if err != nil {
			return fmt.Errorf("failed to parse template %s: %w", templatePath, err)
		}

		// Create directory if it doesn't exist
		dir := filepath.Dir(outputPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		file, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", outputPath, err)
		}
		defer file.Close()

		if err := tmpl.Execute(file, data); err != nil {
			return fmt.Errorf("failed to execute template %s: %w", templatePath, err)
		}

		fmt.Printf("✅ Generated: %s\n", outputPath)
	}

	fmt.Printf("\n🎉 Scaffold generation complete!\n")
	fmt.Printf("💡 Next steps:\n")
	fmt.Printf("   1. Run migrations: astra db:migrate\n")
	fmt.Printf("   2. Start development server: astra dev\n")
	fmt.Printf("   3. Visit your application at http://localhost:3333\n")

	return nil
}

// Helper functions
func toPascalCase(s string) string {
	words := strings.Split(strings.ToLower(s), "_")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.Title(word)
		}
	}
	return strings.Join(words, "")
}

func toSnakeCase(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			result = append(result, '_')
		}
		result = append(result, unicode.ToLower(r))
	}
	return string(result)
}
