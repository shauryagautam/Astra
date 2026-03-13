package commands

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCmd_Smoke verifies that 'astra new' generates a buildable Go project.
func TestNewCmd_Smoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "astra-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	projectName := "testapp"
	projectPath := filepath.Join(tempDir, projectName)

	// We'll call the command logic directly or via exec
	// For simplicity, we'll use exec if 'astra' is in the path,
	// but better to call the Run function with mocked dependencies.

	// Let's call the logic directly by creating a command and setting args
	cmd := NewCmd()
	cmd.SetArgs([]string{projectPath, "--api-only"})

	// Execute the command
	err = cmd.Execute()
	require.NoError(t, err)

	// Verify files exist
	assert.FileExists(t, filepath.Join(projectPath, "main.go"))
	assert.FileExists(t, filepath.Join(projectPath, "go.mod"))
	assert.FileExists(t, filepath.Join(projectPath, "Makefile"))

	// Try to build it
	buildCmd := exec.Command("go", "build", "-o", "/dev/null", "main.go")
	buildCmd.Dir = projectPath
	out, err := buildCmd.CombinedOutput()
	assert.NoError(t, err, "Failed to build generated project: %s", string(out))
}
