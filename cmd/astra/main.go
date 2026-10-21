package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/shauryagautam/Astra/internal/scaffold/tpl"
	"github.com/shauryagautam/Astra/pkg/database"
	"github.com/shauryagautam/Astra/pkg/database/migration"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const modulePath = "github.com/shauryagautam/Astra/cmd/astra"

var ignoredDirectories = map[string]struct{}{
	".git": {}, ".hg": {}, ".svn": {}, "vendor": {}, "bin": {},
	"tmp": {}, "node_modules": {}, "dist": {}, "coverage": {},
}

type commandEnv struct {
	Out    io.Writer
	Err    io.Writer
	Logger *slog.Logger
}

func main() {
	env := commandEnv{Out: os.Stdout, Err: os.Stderr, Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
	os.Exit(run(os.Args[1:], env))
}

func run(args []string, env commandEnv) int {
	if env.Out == nil {
		env.Out = io.Discard
	}
	if env.Err == nil {
		env.Err = io.Discard
	}
	if env.Logger == nil {
		env.Logger = slog.New(slog.NewTextHandler(env.Err, nil))
	}
	if len(args) == 0 {
		printUsage(env.Out)
		return 2
	}

	var err error
	switch args[0] {
	case "new":
		err = newCommand(args[1:], env)
	case "dev":
		err = devCommand(args[1:], env)
	case "make:controller":
		err = generateCommand("controller", args[1:], env)
	case "make:middleware":
		err = generateCommand("middleware", args[1:], env)
	case "make:model":
		err = generateCommand("model", args[1:], env)
	case "make:migration":
		err = generateCommand("migration", args[1:], env)
	case "migrate":
		err = migrateCommand(args[1:], env)
	case "seed":
		err = seedCommand(args[1:], env)
	case "key:generate":
		err = keyCommand(false, args[1:], env)
	case "key:rotate":
		err = keyCommand(true, args[1:], env)
	case "doctor":
		err = doctorCommand(env)
	case "help", "--help", "-h":
		printUsage(env.Out)
		return 0
	default:
		fmt.Fprintf(env.Err, "astra: unknown command %q\n\n", args[0])
		printUsage(env.Err)
		return 2
	}
	if err != nil {
		env.Logger.Error("command failed", "command", args[0], "error", err)
		return 1
	}
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Astra CLI — native Go tooling for Astra applications")
	fmt.Fprintln(w, "\nUsage: astra <command> [arguments]")
	fmt.Fprintln(w, "\nCommands:")
	fmt.Fprintln(w, "  new <module> [--library]       Create a Go application or library")
	fmt.Fprintln(w, "  dev                            Build and reload the current project")
	fmt.Fprintln(w, "  migrate up|down|status|create  Manage database migrations")
	fmt.Fprintln(w, "  seed                           Run registered application data sets")
	fmt.Fprintln(w, "  key:generate|key:rotate        Manage APP_KEY in .env")
	fmt.Fprintln(w, "  doctor                         Check Go, module, and database setup")
	fmt.Fprintln(w, "  make:controller <name>          Generate an HTTP controller")
	fmt.Fprintln(w, "  make:middleware <name>          Generate HTTP middleware")
	fmt.Fprintln(w, "  make:model <name>               Generate a persistence model")
	fmt.Fprintln(w, "  make:migration <name>           Generate a SQL migration")
	fmt.Fprintln(w, "\nInstall: go install "+modulePath+"@latest")
}

// Compatibility wrappers used by older integrations and tests.
func handleNew() { logCompatibilityError(newCommand(os.Args[2:], defaultEnv())) }
func handleDev() { logCompatibilityError(devCommand(os.Args[2:], defaultEnv())) }
func handleMakeController() {
	logCompatibilityError(generateCommand("controller", os.Args[2:], defaultEnv()))
}
func handleMakeMiddleware() {
	logCompatibilityError(generateCommand("middleware", os.Args[2:], defaultEnv()))
}
func handleMakeModel() { logCompatibilityError(generateCommand("model", os.Args[2:], defaultEnv())) }
func handleMakeMigration() {
	logCompatibilityError(generateCommand("migration", os.Args[2:], defaultEnv()))
}

func logCompatibilityError(err error) {
	if err != nil {
		slog.Default().Error("command failed", "error", err)
	}
}

func defaultEnv() commandEnv {
	return commandEnv{Out: os.Stdout, Err: os.Stderr, Logger: slog.New(slog.NewTextHandler(os.Stderr, nil))}
}

type scaffoldOptions struct {
	Module  string
	Library bool
	APIOnly bool
}

func newCommand(args []string, env commandEnv) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(env.Err)
	library := fs.Bool("library", false, "create a reusable library instead of a full application")
	apiOnly := fs.Bool("api-only", false, "create an HTTP API application")
	// Accept flags before or after the module path, which is friendlier for
	// shell completion and unlike flag.FlagSet's default stop-at-first-arg rule.
	normalized := make([]string, 0, len(args))
	positionals := make([]string, 0, 1)
	for _, arg := range args {
		switch arg {
		case "--library", "--api-only":
			normalized = append(normalized, arg)
		default:
			positionals = append(positionals, arg)
		}
	}
	normalized = append(normalized, positionals...)
	if err := fs.Parse(normalized); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: astra new <module> [--library] [--api-only]")
	}
	return scaffold(scaffoldOptions{Module: fs.Arg(0), Library: *library, APIOnly: *apiOnly}, env)
}

type TemplateContext struct {
	AppName        string
	SecurityConfig string
	AppKey         string
	JWTSecret      string
	APIOnly        bool
	Library        bool
}

type MigrationField struct {
	DBName  string
	SQLType string
}

type MigrationContext struct {
	TableName string
	Fields    []MigrationField
}

func scaffold(opts scaffoldOptions, env commandEnv) error {
	if opts.Module == "" || filepath.IsAbs(opts.Module) {
		return errors.New("module must be a non-empty relative path")
	}
	root := opts.Module
	dirs := []string{root, filepath.Join(root, "database", "migrations")}
	if !opts.Library {
		dirs = append(dirs,
			filepath.Join(root, "handler"),
			filepath.Join(root, "model"),
			filepath.Join(root, "middleware"),
			filepath.Join(root, "database", "data"),
		)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	ctx := TemplateContext{
		AppName: opts.Module, SecurityConfig: "app.Env().IsProd()",
		AppKey: randomKey(), JWTSecret: randomKey(), APIOnly: opts.APIOnly, Library: opts.Library,
	}
	if opts.Library {
		if err := renderTemplate("library.go.tmpl", filepath.Join(root, "library.go"), ctx); err != nil {
			return err
		}
	} else {
		templates := map[string]string{
			"main.go.tmpl":       filepath.Join(root, "main.go"),
			"wire.go.tmpl":       filepath.Join(root, "wire.go"),
			"wire_gen.go.tmpl":   filepath.Join(root, "wire_gen.go"),
			"readme.tmpl":        filepath.Join(root, "README.md"),
			"makefile.tmpl":      filepath.Join(root, "Makefile"),
			"env.tmpl":           filepath.Join(root, ".env"),
			"gitignore.tmpl":     filepath.Join(root, ".gitignore"),
			"routes.api.go.tmpl": filepath.Join(root, "routes.go"),
		}
		for source, destination := range templates {
			if err := renderTemplateWithData(source, destination, ctx); err != nil {
				return err
			}
		}
	}
	migrationPath := filepath.Join(root, "database", "migrations",
		time.Now().Format("20060102150405")+"_"+slug("create users table")+".sql")
	if err := renderTemplateWithData("migration.sql.tmpl", migrationPath, MigrationContext{
		TableName: "users", Fields: []MigrationField{{DBName: "name", SQLType: "VARCHAR(255)"}},
	}); err != nil {
		return err
	}
	if err := runCommand(root, env, "go", "mod", "init", opts.Module); err != nil {
		return err
	}
	if err := runCommand(root, env, "go", "mod", "tidy"); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); errors.Is(err, os.ErrNotExist) {
		if err := runCommand(root, env, "git", "init"); err != nil {
			return err
		}
	}
	fmt.Fprintf(env.Out, "created %s\n", root)
	return nil
}

func randomKey() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawStdEncoding.EncodeToString(buf)
}

func renderTemplate(name, destination string, data any) error {
	return renderTemplateWithData(name, destination, data)
}

func renderTemplateWithData(name, destination string, data any) error {
	content, err := fs.ReadFile(tpl.FS, name)
	if err != nil {
		return fmt.Errorf("read template %s: %w", name, err)
	}
	t, err := template.New(name).Funcs(template.FuncMap{"title": cases.Title(language.English).String, "slug": slug}).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", name, err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer file.Close()
	if err := t.Execute(file, data); err != nil {
		return fmt.Errorf("render %s: %w", name, err)
	}
	return nil
}

func runCommand(dir string, env commandEnv, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, env.Out, env.Err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if b.Len() > 0 && !strings.HasSuffix(b.String(), "_") {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func exportName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' || r == ' ' || r == '/' })
	var result string
	for _, part := range parts {
		result += cases.Title(language.English).String(strings.ToLower(part))
	}
	if result == "" {
		return "Generated"
	}
	return result
}

func generateCommand(kind string, args []string, env commandEnv) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: astra make:%s <name>", kind)
	}
	name := args[0]
	exported := exportName(name)
	lower := slug(name)
	switch kind {
	case "controller":
		dir, pkg := generatorDestination("handler", "handler", lower+"_controller.go")
		return renderTemplateWithData("controller.go.tmpl", filepath.Join(dir, lower+"_controller.go"),
			struct{ Name, NameLower, TableName, Package string }{exported, lower, lower + "s", pkg})
	case "middleware":
		dir, pkg := generatorDestination("middleware", "middleware", lower+".go")
		return renderTemplateWithData("middleware.go.tmpl", filepath.Join(dir, lower+".go"),
			struct{ Name, NameLower, Package string }{exported, lower, pkg})
	case "model":
		dir, pkg := generatorDestination("model", "model", lower+".go")
		return renderTemplateWithData("model.go.tmpl", filepath.Join(dir, lower+".go"),
			struct{ Name, NameLower, Package string }{exported, lower, pkg})
	case "migration":
		dir := "database/migrations"
		if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
			dir = "."
		}
		path := filepath.Join(dir, time.Now().Format("20060102150405")+"_"+lower+".sql")
		return renderTemplateWithData("migration.sql.tmpl", path, MigrationContext{TableName: lower, Fields: []MigrationField{{DBName: "name", SQLType: "VARCHAR(255)"}}})
	default:
		return fmt.Errorf("unsupported generator %q", kind)
	}
}

func generatorDestination(preferred, packageName, filename string) (string, string) {
	if _, err := os.Stat(preferred); err == nil {
		return preferred, packageName
	}
	return ".", "main"
}

func migrateCommand(args []string, env commandEnv) error {
	if len(args) == 0 {
		return errors.New("usage: astra migrate up|down|status|create [name]")
	}
	if args[0] == "create" {
		if len(args) != 2 {
			return errors.New("usage: astra migrate create <name>")
		}
		return generateCommand("migration", args[1:], env)
	}
	if len(args) != 1 {
		return errors.New("usage: astra migrate up|down|status")
	}
	_, runner, cleanup, err := migrationRunner()
	if err != nil {
		return err
	}
	defer cleanup()
	ctx := context.Background()
	switch args[0] {
	case "up":
		return runner.Run(ctx)
	case "down":
		return runner.Rollback(ctx)
	case "status":
		applied, pending, err := runner.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(env.Out, "applied: %d\npending: %d\n", len(applied), len(pending))
		return nil
	default:
		return fmt.Errorf("unknown migration action %q", args[0])
	}
}

func migrationRunner() (*database.DB, *migration.Runner, func(), error) {
	raw, err := config.Load(".env")
	if err != nil {
		return nil, nil, nil, err
	}
	driver := raw.String("DB_DRIVER", raw.String("DB_CONNECTION", "postgres"))
	dsn := raw.String("DB_DSN", raw.String("DATABASE_URL", ""))
	db, err := database.Open(database.Config{Driver: driver, DSN: dsn})
	if err != nil {
		return nil, nil, nil, err
	}
	dialect, _ := database.ResolveDialect(driver)
	return db, migration.NewRunner(db.Pool(), dialect, "database/migrations", nil), func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Default().Error("close database", "error", closeErr)
		}
	}, nil
}

func seedCommand(args []string, env commandEnv) error {
	if len(args) != 0 {
		return errors.New("usage: astra seed")
	}
	entries, err := os.ReadDir("database/data")
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(env.Out, "no data sets found in database/data")
		return nil
	}
	if err != nil {
		return err
	}
	db, _, cleanup, err := migrationRunner()
	if err != nil {
		return err
	}
	defer cleanup()
	ctx := context.Background()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		path := filepath.Join("database/data", entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read data set %s: %w", path, err)
		}
		if _, err := db.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("run data set %s: %w", path, err)
		}
		fmt.Fprintf(env.Out, "loaded %s\n", entry.Name())
	}
	return nil
}

func keyCommand(rotate bool, args []string, env commandEnv) error {
	if len(args) != 0 {
		return errors.New("usage: astra key:generate")
	}
	path := ".env"
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lines := strings.Split(string(content), "\n")
	key := randomKey()
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "APP_KEY=") {
			lines[i], found = "APP_KEY="+key, true
		}
	}
	if !found {
		lines = append(lines, "APP_KEY="+key)
	}
	if err := os.WriteFile(path, []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n")+"\n"), 0o600); err != nil {
		return err
	}
	action := "generated"
	if rotate {
		action = "rotated"
	}
	fmt.Fprintf(env.Out, "APP_KEY %s in %s\n", action, path)
	return nil
}

func doctorCommand(env commandEnv) error {
	for _, tool := range []string{"go", "git"} {
		path, err := exec.LookPath(tool)
		if err != nil {
			return fmt.Errorf("%s is not available: %w", tool, err)
		}
		fmt.Fprintf(env.Out, "%s: %s\n", tool, path)
	}
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("go.mod: %w", err)
	}
	fmt.Fprintln(env.Out, "doctor: OK")
	return nil
}

func devCommand(args []string, env commandEnv) error {
	if len(args) != 0 {
		return errors.New("usage: astra dev")
	}
	if _, err := os.Stat("main.go"); err != nil {
		return fmt.Errorf("main.go: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return watchAndRunContext(ctx, env)
}

func watchAndRun(ctx context.Context, envs ...commandEnv) {
	env := defaultEnv()
	if len(envs) > 0 {
		env = envs[0]
	}
	if err := watchAndRunContext(ctx, env); err != nil {
		env.Logger.Error("development server stopped", "error", err)
	}
}

func watchAndRunContext(ctx context.Context, env commandEnv) error {
	watcher, err := newWatcher(".")
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := watcher.Close(); closeErr != nil {
			env.Logger.Error("close file watcher", "error", closeErr)
		}
	}()
	var process *exec.Cmd
	stopProcess := func() error {
		if process == nil || process.Process == nil {
			return nil
		}
		if err := process.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
			if killErr := process.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
				env.Logger.Error("kill application", "error", killErr)
			}
		}
		err := process.Wait()
		process = nil
		return err
	}
	restart := func() error {
		if stopErr := stopProcess(); stopErr != nil {
			env.Logger.Debug("application stopped", "error", stopErr)
		}
		binary, err := buildApplicationWithEnv(ctx, env)
		if err != nil {
			return err
		}
		process = exec.CommandContext(ctx, binary)
		process.Stdout, process.Stderr = env.Out, env.Err
		return process.Start()
	}
	if err := restart(); err != nil {
		env.Logger.Error("initial build failed", "error", err)
	}
	debounce := time.NewTimer(time.Hour)
	if !debounce.Stop() {
		<-debounce.C
	}
	pending := false
	for {
		select {
		case <-ctx.Done():
			if stopErr := stopProcess(); stopErr != nil {
				env.Logger.Debug("application stopped", "error", stopErr)
			}
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return errors.New("file watcher closed")
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 || ignoredPath(event.Name) {
				continue
			}
			pending = true
			debounce.Reset(150 * time.Millisecond)
			if event.Op&fsnotify.Create != 0 {
				if watchErr := addWatchTree(watcher, event.Name); watchErr != nil {
					env.Logger.Error("watch new directory", "path", event.Name, "error", watchErr)
				}
			}
		case <-debounce.C:
			if pending {
				pending = false
				if err := restart(); err != nil {
					env.Logger.Error("reload failed", "error", err)
				}
			}
		case err, ok := <-watcher.Errors:
			if ok {
				env.Logger.Error("watcher error", "error", err)
			}
		}
	}
}

func newWatcher(root string) (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := addWatchTree(w, root); err != nil {
		if closeErr := w.Close(); closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return w, nil
}

func addWatchTree(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			if path != root && ignoredRelative(relative) {
				return filepath.SkipDir
			}
			return w.Add(path)
		}
		return nil
	})
}

func ignoredPath(path string) bool {
	return ignoredRelative(filepath.Clean(path))
}

func ignoredRelative(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if _, ok := ignoredDirectories[part]; ok {
			return true
		}
		if len(part) > 1 && part[0] == '.' {
			return true
		}
	}
	return false
}

func buildApplication(ctx context.Context) (string, error) {
	return buildApplicationWithEnv(ctx, defaultEnv())
}

func buildApplicationWithEnv(ctx context.Context, env commandEnv) (string, error) {
	if err := os.MkdirAll("bin", 0o755); err != nil {
		return "", fmt.Errorf("create bin: %w", err)
	}
	name := "dev_app"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join("bin", name)
	if err := runCommand(".", env, "go", "build", "-o", path, "."); err != nil {
		return "", err
	}
	return path, nil
}

type fileState struct {
	modTime time.Time
	size    int64
	mode    fs.FileMode
}

func scanFiles(root string, files map[string]fileState) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() && path != root && ignoredRelative(relative) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		switch filepath.Ext(path) {
		case ".go", ".html", ".tmpl", ".env":
			// source file
			info, err := d.Info()
			if err != nil {
				return err
			}
			files[path] = fileState{modTime: info.ModTime(), size: info.Size(), mode: info.Mode()}
		}
		return nil
	})
}

func filesChanged(previous, current map[string]fileState) bool {
	if len(previous) != len(current) {
		return true
	}
	for path, state := range current {
		if old, ok := previous[path]; !ok || old != state {
			return true
		}
	}
	return false
}

func sortedNames(m map[string]string) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
