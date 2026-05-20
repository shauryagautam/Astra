package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/shauryagautam/Astra/internal/scaffold/tpl"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "new":
		handleNew()
	case "dev":
		handleDev()
	case "make:controller":
		handleMakeController()
	case "make:middleware":
		handleMakeMiddleware()
	case "make:model":
		handleMakeModel()
	case "make:migration":
		handleMakeMigration()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Astra CLI Tool - Production Grade Developer Assistant")
	fmt.Println("\nUsage:")
	fmt.Println("  astra <command> [arguments]")
	fmt.Println("\nCommands:")
	fmt.Println("  new <app_name>        Scaffold a new Astra application layout")
	fmt.Println("  dev                   Run application with live auto-reload on file changes")
	fmt.Println("  make:controller <name> Generate a new HTTP resource controller")
	fmt.Println("  make:middleware <name> Generate a new HTTP middleware")
	fmt.Println("  make:model <name>      Generate a new GORM-compatible database schema model")
	fmt.Println("  make:migration <name>  Generate a new SQL database schema migration version")
	fmt.Println("\nFor more details, visit: https://github.com/shauryagautam/Astra")
}

// ─── Scaffolding Generator (new) ─────────────────────────────────────────────

type TemplateContext struct {
	AppName        string
	SecurityConfig string
	AppKey         string
	JWTSecret      string
}

type MigrationField struct {
	DBName  string
	SQLType string
}

type MigrationContext struct {
	TableName string
	Fields    []MigrationField
}

func generateRandomKey(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func handleNew() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Project name is required.")
		fmt.Println("Usage: astra new <app_name> [--api-only]")
		os.Exit(1)
	}

	appName := os.Args[2]
	apiOnly := false
	for _, arg := range os.Args[3:] {
		if arg == "--api-only" {
			apiOnly = true
		}
	}

	fmt.Printf("🚀 Scaffolding new Astra application: %s (API Only: %t)\n", appName, apiOnly)

	// Create directory structure
	dirs := []string{
		appName,
		filepath.Join(appName, "app", "handler"),
		filepath.Join(appName, "app", "schema"),
		filepath.Join(appName, "app", "jobs"),
		filepath.Join(appName, "database", "migrations"),
		filepath.Join(appName, "database", "seeders"),
		filepath.Join(appName, "internal", "routes"),
		filepath.Join(appName, "shared", "astra-client"),
		filepath.Join(appName, "config"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", d, err)
		}
	}

	// Template mappings (embedded path -> destination path)
	templates := map[string]string{
		"main.go.tmpl":              filepath.Join(appName, "main.go"),
		"wire.go.tmpl":              filepath.Join(appName, "wire.go"),
		"makefile.tmpl":             filepath.Join(appName, "Makefile"),
		"env.tmpl":                  filepath.Join(appName, ".env"),
		"gitignore.tmpl":            filepath.Join(appName, ".gitignore"),
		"readme.tmpl":               filepath.Join(appName, "README.md"),
		"docker-compose.yml.tmpl":   filepath.Join(appName, "docker-compose.yml"),
		"Dockerfile.tmpl":           filepath.Join(appName, "Dockerfile"),
		"fly.toml.tmpl":             filepath.Join(appName, "fly.toml"),
		"routes.api.go.tmpl":        filepath.Join(appName, "internal", "routes", "routes.go"),
	}

	ctx := TemplateContext{
		AppName:        appName,
		SecurityConfig: "app.Env().IsProd()",
		AppKey:         generateRandomKey(32),
		JWTSecret:      generateRandomKey(32),
	}

	for src, dst := range templates {
		renderEmbeddedTemplate(tpl.FS, src, dst, ctx)
	}

	// Create placeholder file in migrations
	dummyMigration := filepath.Join(appName, "database", "migrations", fmt.Sprintf("%s_create_users_table.sql", time.Now().Format("20060102150405")))
	renderEmbeddedTemplate(tpl.FS, "migration.sql.tmpl", dummyMigration, MigrationContext{
		TableName: "users",
		Fields: []MigrationField{
			{DBName: "name", SQLType: "VARCHAR(255)"},
		},
	})

	// Create placeholder files
	_ = os.WriteFile(filepath.Join(appName, "app", "handler", ".gitkeep"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(appName, "app", "schema", ".gitkeep"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(appName, "app", "jobs", ".gitkeep"), []byte(""), 0644)

	// Run mod init and tidy
	fmt.Println("📦 Initializing Go modules and packages...")
	runCmd(appName, "go", "mod", "init", appName)
	runCmd(appName, "go", "mod", "tidy")

	// Initialize git
	runCmd(appName, "git", "init")

	fmt.Println("\n✨ Application scaffolded successfully!")
	fmt.Printf("👉 Next steps:\n   cd %s\n   make setup\n   astra dev\n", appName)
}

func renderEmbeddedTemplate(fsys embed.FS, srcPath, dstPath string, data any) {
	tmplBytes, err := fs.ReadFile(fsys, srcPath)
	if err != nil {
		log.Fatalf("Failed to read template %s: %v", srcPath, err)
	}

	tmpl, err := template.New(srcPath).Parse(string(tmplBytes))
	if err != nil {
		log.Fatalf("Failed to parse template %s: %v", srcPath, err)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		log.Fatalf("Failed to create destination file %s: %v", dstPath, err)
	}
	defer dstFile.Close()

	if err := tmpl.Execute(dstFile, data); err != nil {
		log.Fatalf("Failed to render template %s: %v", srcPath, err)
	}
	fmt.Printf("   Generated: %s\n", dstPath)
}

func runCmd(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// ─── Live Auto-Reload (dev) ──────────────────────────────────────────────────

func handleDev() {
	fmt.Println("🔄 Starting Astra Live-Reload Server...")

	// Verify we are in an Astra project root
	if _, err := os.Stat("main.go"); os.IsNotExist(err) {
		log.Fatalf("Error: 'main.go' not found. Please run 'astra dev' inside the project root folder.")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Stopping live-reload server...")
		cancel()
	}()

	watchAndRun(ctx)
}

func watchAndRun(ctx context.Context) {
	var cmd *exec.Cmd

	// Function to stop the active process
	stopProcess := func() {
		if cmd != nil && cmd.Process != nil {
			// Send SIGTERM to allow graceful shutdown
			_ = cmd.Process.Signal(syscall.SIGTERM)
			
			// Wait or kill
			done := make(chan struct{})
			go func() {
				_ = cmd.Wait()
				close(done)
			}()
			
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
			}
			cmd = nil
		}
	}

	// Function to compile and launch
	restart := func() {
		stopProcess()

		fmt.Println("🛠️  Compiling codebase changes...")
		binaryPath := filepath.Join("bin", "dev_app")
		build := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "main.go")
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr

		if err := build.Run(); err != nil {
			fmt.Println("❌ Compilation Failed. Fix issues to trigger rebuild.")
			return
		}

		fmt.Println("🚀 Restarting Application Server...")
		cmd = exec.CommandContext(ctx, binaryPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Start(); err != nil {
			fmt.Printf("❌ Failed to start application: %v\n", err)
		}
	}

	// Initial compile and run
	restart()

	// Watch loops
	filesMap := make(map[string]time.Time)
	
	// Pre-fill file map
	scanFiles(filesMap)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			stopProcess()
			return
		case <-ticker.C:
			changed := false
			currentMap := make(map[string]time.Time)
			scanFiles(currentMap)

			// Check for edits or additions
			for path, modTime := range currentMap {
				if oldTime, ok := filesMap[path]; !ok || !oldTime.Equal(modTime) {
					changed = true
					break
				}
			}

			// Check for deletions
			if !changed {
				for path := range filesMap {
					if _, ok := currentMap[path]; !ok {
						changed = true
						break
					}
				}
			}

			if changed {
				fmt.Println("📝 Source file changes detected.")
				filesMap = currentMap
				restart()
			}
		}
	}
}

func scanFiles(m map[string]time.Time) {
	_ = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Skip hidden folders and vendors
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "bin" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext == ".go" || ext == ".html" || ext == ".tmpl" || ext == ".env" {
			if info, err := d.Info(); err == nil {
				m[path] = info.ModTime()
			}
		}
		return nil
	})
}

// ─── Generators (make:controller, make:middleware, make:model, make:migration)

func handleMakeController() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Controller name is required.")
		fmt.Println("Usage: astra make:controller <Name>")
		os.Exit(1)
	}

	name := strings.Title(os.Args[2])
	nameLower := strings.ToLower(name)

	// Determine package
	pkg := "handler"
	if _, err := os.Stat(filepath.Join("app", "handler")); os.IsNotExist(err) {
		pkg = "main"
	}

	dst := filepath.Join("app", "handler", nameLower+"_controller.go")
	if pkg == "main" {
		dst = nameLower + "_controller.go"
	}

	data := struct {
		Name      string
		NameLower string
		TableName string
		Package   string
	}{
		Name:      name,
		NameLower: nameLower,
		TableName: nameLower + "s",
		Package:   pkg,
	}

	renderEmbeddedTemplate(tpl.FS, "controller.go.tmpl", dst, data)
	fmt.Printf("✅ Controller generated successfully at %s\n", dst)
}

func handleMakeMiddleware() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Middleware name is required.")
		fmt.Println("Usage: astra make:middleware <Name>")
		os.Exit(1)
	}

	name := os.Args[2]
	nameLower := strings.ToLower(name)

	pkg := "middleware"
	dst := filepath.Join("app", "middleware", nameLower+".go")
	if _, err := os.Stat(filepath.Join("app", "middleware")); os.IsNotExist(err) {
		pkg = "main"
		dst = nameLower + "_middleware.go"
	}

	data := struct {
		Name      string
		NameLower string
		Package   string
	}{
		Name:      strings.Title(name),
		NameLower: nameLower,
		Package:   pkg,
	}

	renderEmbeddedTemplate(tpl.FS, "middleware.go.tmpl", dst, data)
	fmt.Printf("✅ Middleware generated successfully at %s\n", dst)
}

type ModelField struct {
	FieldName string
	GoType    string
	SQLType   string
	JSONName  string
}

func handleMakeModel() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Model name is required.")
		fmt.Println("Usage: astra make:model <Name>")
		os.Exit(1)
	}

	name := strings.Title(os.Args[2])
	nameLower := strings.ToLower(name)

	pkg := "schema"
	dst := filepath.Join("app", "schema", nameLower+".go")
	if _, err := os.Stat(filepath.Join("app", "schema")); os.IsNotExist(err) {
		pkg = "main"
		dst = nameLower + "_schema.go"
	}

	data := struct {
		Name      string
		NameLower string
		Package   string
		Fields    []ModelField
	}{
		Name:      name,
		NameLower: nameLower,
		Package:   pkg,
		Fields: []ModelField{
			{FieldName: "Name", GoType: "string", SQLType: "varchar(255)", JSONName: "name"},
		},
	}

	renderEmbeddedTemplate(tpl.FS, "schema.go.tmpl", dst, data)
	fmt.Printf("✅ Model schema generated successfully at %s\n", dst)
}

func handleMakeMigration() {
	if len(os.Args) < 3 {
		fmt.Println("Error: Migration name is required.")
		fmt.Println("Usage: astra make:migration <Name>")
		os.Exit(1)
	}

	name := os.Args[2]
	nameLower := strings.ToLower(name)

	dstDir := filepath.Join("database", "migrations")
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		dstDir = "."
	}

	timestamp := time.Now().Format("20060102150405")
	dst := filepath.Join(dstDir, fmt.Sprintf("%s_%s.sql", timestamp, nameLower))

	data := MigrationContext{
		TableName: nameLower,
		Fields: []MigrationField{
			{DBName: "name", SQLType: "VARCHAR(255)"},
		},
	}

	renderEmbeddedTemplate(tpl.FS, "migration.sql.tmpl", dst, data)
	fmt.Printf("✅ Migration file generated successfully at %s\n", dst)
}
