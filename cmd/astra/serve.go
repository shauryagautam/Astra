package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the development server",
	Long:  `Starts the HTTP server. Use --watch for hot-reload during development.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		watch, _ := cmd.Flags().GetBool("watch")
		if watch {
			return serveWithWatch()
		}
		return serveDirect()
	},
}

func init() {
	serveCmd.Flags().BoolP("watch", "w", false, "Enable hot-reload (requires Air)")
}

// serveDirect runs server.go directly.
func serveDirect() error {
	fmt.Println("🚀 Starting server...")
	goCmd := exec.Command("go", "run", "server.go")
	goCmd.Stdout = os.Stdout
	goCmd.Stderr = os.Stderr
	goCmd.Stdin = os.Stdin

	if err := goCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	if goCmd.Process != nil {
		_ = goCmd.Process.Signal(syscall.SIGTERM)
		_ = goCmd.Wait()
	}
	return nil
}

// serveWithWatch runs with Air for hot-reload.
func serveWithWatch() error {
	// Check if air is available
	if _, err := exec.LookPath("air"); err != nil {
		fmt.Println("⚠️  Air not found. Install it:")
		fmt.Println("   go install github.com/air-verse/air@latest")
		fmt.Println()
		fmt.Println("Falling back to direct mode (no hot-reload)...")
		return serveDirect()
	}

	// Generate .air.toml if not present
	if _, err := os.Stat(".air.toml"); os.IsNotExist(err) {
		if err := generateAirConfig(); err != nil {
			return err
		}
	}

	fmt.Println("🔥 Starting server with hot-reload (Air)...")
	airCmd := exec.Command("air")
	airCmd.Stdout = os.Stdout
	airCmd.Stderr = os.Stderr
	airCmd.Stdin = os.Stdin

	return airCmd.Run()
}

// generateAirConfig creates an .air.toml config file.
func generateAirConfig() error {
	config := `# Air configuration for Astra Go hot-reload
root = "."
tmp_dir = "tmp"

[build]
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main server.go"
  delay = 1000
  exclude_dir = ["tmp", "vendor", "node_modules"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  include_dir = []
  include_ext = ["go"]
  kill_delay = "0s"
  log = "build-errors.log"
  send_interrupt = true
  stop_on_error = true

[log]
  time = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[misc]
  clean_on_exit = true
`
	if err := os.WriteFile(".air.toml", []byte(config), 0644); err != nil {
		return fmt.Errorf("failed to create .air.toml: %w", err)
	}
	fmt.Println("✅ Generated .air.toml")
	return nil
}
