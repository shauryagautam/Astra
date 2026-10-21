package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunReturnsUsefulExitCodes(t *testing.T) {
	var out, errOut bytes.Buffer
	env := commandEnv{Out: &out, Err: &errOut, Logger: slog.New(slog.NewTextHandler(&errOut, nil))}
	if code := run([]string{"help"}, env); code != 0 {
		t.Fatalf("help exit code = %d", code)
	}
	if code := run([]string{"does-not-exist"}, env); code != 2 {
		t.Fatalf("unknown command exit code = %d", code)
	}
	if code := run([]string{"make:model"}, env); code != 1 {
		t.Fatalf("invalid generator exit code = %d", code)
	}
}

func TestKeyCommandsCreateAndRotateEnvFile(t *testing.T) {
	projectDir := t.TempDir()
	withWorkingDirectory(t, projectDir)
	var output bytes.Buffer
	env := commandEnv{Out: &output, Err: &output, Logger: slog.New(slog.NewTextHandler(&output, nil))}

	if err := keyCommand(false, nil, env); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(projectDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if err := keyCommand(true, nil, env); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(projectDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) || !bytes.Contains(second, []byte("APP_KEY=")) {
		t.Fatal("key rotate did not replace APP_KEY")
	}
}

func TestGeneratorsUseGoNamesAndDoNotOverwrite(t *testing.T) {
	projectDir := t.TempDir()
	withWorkingDirectory(t, projectDir)
	if err := os.MkdirAll(filepath.Join("model"), 0o755); err != nil {
		t.Fatal(err)
	}
	env := commandEnv{Out: io.Discard, Err: io.Discard, Logger: slog.Default()}
	if err := generateCommand("model", []string{"user-profile"}, env); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("model", "user_profile.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(content, []byte("type UserProfile struct")) {
		t.Fatalf("generated model has unexpected content:\n%s", content)
	}
	if err := generateCommand("model", []string{"user-profile"}, env); err == nil {
		t.Fatal("generator overwrote an existing file")
	}
}

func TestNewWatcherSkipsIgnoredDirectories(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"app", "vendor", "bin", ".git"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	watcher, err := newWatcher(root)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()
	watched := make(map[string]bool)
	for _, path := range watcher.WatchList() {
		watched[path] = true
	}
	if !watched[filepath.Join(root, "app")] {
		t.Fatal("source directory was not watched")
	}
	for _, dir := range []string{"vendor", "bin", ".git"} {
		if watched[filepath.Join(root, dir)] {
			t.Fatalf("ignored directory %s was watched", dir)
		}
	}

}

func TestLibraryScaffoldIsMinimalAndBuildable(t *testing.T) {
	projectDir := t.TempDir()
	withWorkingDirectory(t, projectDir)
	env := commandEnv{Out: io.Discard, Err: io.Discard, Logger: slog.Default()}
	if err := newCommand([]string{"example.com/library", "--library"}, env); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "example.com", "library", "library.go")); err != nil {
		t.Fatalf("library scaffold missing library.go: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectDir, "example.com", "library", "main.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("library scaffold unexpectedly created an application main.go")
	}
}

func TestNewCreatesBuildableAndRunnableScaffold(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	projectRoot := t.TempDir()
	modulePath := "example.com/clean-scaffold"

	run := func(dir string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		var output bytes.Buffer
		cmd.Stdout = &output
		cmd.Stderr = &output
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, output.String())
		}
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	originalArgs := os.Args
	defer func() {
		_ = os.Chdir(originalDir)
		os.Args = originalArgs
	}()
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"astra", "new", modulePath}
	handleNew()
	appDir := filepath.Join(projectRoot, filepath.FromSlash(modulePath))

	run(appDir, "go", "mod", "edit", "-replace", "github.com/shauryagautam/Astra="+repoRoot)
	run(appDir, "go", "mod", "tidy")
	binaryPath := filepath.Join(appDir, "bin", "app")
	run(appDir, "go", "build", "-o", binaryPath, ".")

	cmd := exec.Command(binaryPath)
	cmd.Dir = appDir
	cmd.Env = append(os.Environ(),
		"DB_DRIVER=sqlite",
		"DB_CONNECTION=sqlite",
		"DB_DSN=:memory:",
		"DATABASE_URL=:memory:",
		"PORT=3334",
	)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting scaffold failed: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			_ = cmd.Process.Kill()
		}
	}()

	select {
	case err := <-done:
		t.Fatalf("scaffold exited before starting: %v\n%s", err, output.String())
	case <-time.After(2 * time.Second):
		return
	}
}

func TestNewAPIOnlyChangesGeneratedFilesAndMakeTargets(t *testing.T) {
	projectRoot := t.TempDir()
	withWorkingDirectory(t, projectRoot)
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"astra", "new", "standard-app"}
	handleNew()
	os.Args = []string{"astra", "new", "api-app", "--api-only"}
	handleNew()

	standardReadme, err := os.ReadFile(filepath.Join(projectRoot, "standard-app", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	apiReadme, err := os.ReadFile(filepath.Join(projectRoot, "api-app", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(standardReadme, apiReadme) {
		t.Fatal("API-only scaffold README did not differ from the standard scaffold")
	}
	if !strings.Contains(string(apiReadme), "API-only") {
		t.Fatal("API-only scaffold README does not describe its API-only mode")
	}

	makefile, err := os.ReadFile(filepath.Join(projectRoot, "api-app", "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	makeText := string(makefile)
	for _, expected := range []string{
		"@mkdir -p bin",
		"@go build -o bin/$(APP_NAME) .",
		"@go run . migrate up",
		"@go run . migrate down",
	} {
		if !strings.Contains(makeText, expected) {
			t.Errorf("Makefile is missing %q", expected)
		}
	}
}

func TestGeneratedMigrationCommandsRunUpAndDown(t *testing.T) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to locate test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))
	projectRoot := t.TempDir()
	withWorkingDirectory(t, projectRoot)
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	os.Args = []string{"astra", "new", "migration-app"}
	handleNew()
	appDir := filepath.Join(projectRoot, "migration-app")
	migrationPath := filepath.Join(appDir, "database", "migrations")
	entries, err := os.ReadDir(migrationPath)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one generated migration: entries=%v err=%v", entries, err)
	}
	writeTestFile(t, filepath.Join(migrationPath, entries[0].Name()), `-- +migrate Up
CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY);

-- +migrate Down
DROP TABLE IF EXISTS users;
`)

	runTestCommand(t, appDir, "go", "mod", "edit", "-replace", "github.com/shauryagautam/Astra="+repoRoot)
	runTestCommand(t, appDir, "go", "mod", "tidy")
	runTestCommand(t, appDir, "go", "build", "-o", filepath.Join(appDir, "bin", "app"), ".")

	dbPath := filepath.Join(appDir, "test.db")
	env := append(os.Environ(),
		"DB_DRIVER=sqlite",
		"DB_CONNECTION=sqlite",
		"DB_DSN="+dbPath,
		"DATABASE_URL="+dbPath,
	)
	runTestCommandWithEnv(t, appDir, env, filepath.Join(appDir, "bin", "app"), "migrate", "up")
	runTestCommandWithEnv(t, appDir, env, filepath.Join(appDir, "bin", "app"), "migrate", "down")
}

func TestBuildApplicationBuildsWholePackageAndCreatesBin(t *testing.T) {
	projectDir := t.TempDir()
	writeTestFile(t, filepath.Join(projectDir, "go.mod"), "module example.com/build-test\n\ngo 1.27\n")
	writeTestFile(t, filepath.Join(projectDir, "main.go"), `package main

import "fmt"

func main() { fmt.Println(message()) }
`)
	writeTestFile(t, filepath.Join(projectDir, "message.go"), `package main

func message() string { return "built package" }
`)

	withWorkingDirectory(t, projectDir)
	binaryPath, err := buildApplication(context.Background())
	if err != nil {
		t.Fatalf("buildApplication() failed: %v", err)
	}
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("built binary %q is missing: %v", binaryPath, err)
	}
}

func runTestCommand(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	runTestCommandWithEnv(t, dir, nil, name, args...)
}

func runTestCommandWithEnv(t *testing.T, dir string, env []string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, output.String())
	}
}

func TestScanFilesIncludesRootAndSkipsGeneratedDirectories(t *testing.T) {
	projectDir := t.TempDir()
	writeTestFile(t, filepath.Join(projectDir, "main.go"), "package main\n")
	writeTestFile(t, filepath.Join(projectDir, "app", "handler.go"), "package app\n")
	writeTestFile(t, filepath.Join(projectDir, "bin", "generated.go"), "package generated\n")
	writeTestFile(t, filepath.Join(projectDir, ".hidden", "ignored.go"), "package ignored\n")

	files := make(map[string]fileState)
	if err := scanFiles(projectDir, files); err != nil {
		t.Fatalf("scanFiles() failed: %v", err)
	}

	if _, ok := files[filepath.Join(projectDir, "main.go")]; !ok {
		t.Fatal("scanFiles() did not include a source file at the root")
	}
	if _, ok := files[filepath.Join(projectDir, "app", "handler.go")]; !ok {
		t.Fatal("scanFiles() did not include a nested source file")
	}
	if _, ok := files[filepath.Join(projectDir, "bin", "generated.go")]; ok {
		t.Fatal("scanFiles() included generated files under bin")
	}
	if _, ok := files[filepath.Join(projectDir, ".hidden", "ignored.go")]; ok {
		t.Fatal("scanFiles() included files under hidden directories")
	}
}

func TestFilesChangedDetectsEditsAdditionsAndDeletions(t *testing.T) {
	original := fileState{modTime: time.Unix(1, 0), size: 10, mode: 0o644}
	edited := fileState{modTime: original.modTime, size: 11, mode: original.mode}

	if filesChanged(map[string]fileState{"main.go": original}, map[string]fileState{"main.go": original}) {
		t.Fatal("filesChanged() reported an unchanged tree as changed")
	}
	if !filesChanged(map[string]fileState{"main.go": original}, map[string]fileState{"main.go": edited}) {
		t.Fatal("filesChanged() missed an edited file")
	}
	if !filesChanged(map[string]fileState{}, map[string]fileState{"main.go": original}) {
		t.Fatal("filesChanged() missed an added file")
	}
	if !filesChanged(map[string]fileState{"main.go": original}, map[string]fileState{}) {
		t.Fatal("filesChanged() missed a deleted file")
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func withWorkingDirectory(t *testing.T, dir string) {
	t.Helper()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}
