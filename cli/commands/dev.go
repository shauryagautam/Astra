package commands

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func DevCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dev",
		Short: "Start the development server with live reload",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting Astra development server...")

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				fmt.Println("Failed to initialize file watcher:", err)
				return
			}
			defer watcher.Close()

			// Add .go files
			// Add files to watcher, ignoring node_modules, .git, etc.
			err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					dir := info.Name()
					if dir == "node_modules" || dir == ".git" || dir == ".astra" || dir == "vendor" {
						return filepath.SkipDir
					}
					return watcher.Add(path)
				}
				return nil
			})
			if err != nil {
				fmt.Println("Failed to watch directories:", err)
				return
			}

			if err := watcher.Add("."); err != nil {
				fmt.Printf("Failed to add watcher: %v\n", err)
			}

			var (
				mu      sync.Mutex
				process *exec.Cmd
				buildCb *time.Timer
			)

			startProcess := func() {
				mu.Lock()
				defer mu.Unlock()

				if process != nil && process.Process != nil {
					if err := process.Process.Signal(syscall.SIGTERM); err != nil {
						fmt.Printf("[Astra Dev] Failed to send SIGTERM: %v\n", err)
					}
					// Wait with timeout
					done := make(chan error, 1)
					go func() { done <- process.Wait() }()
					select {
					case <-done:
					case <-time.After(2 * time.Second):
						_ = process.Process.Kill()
					}
				}

				fmt.Println("\n[Astra Dev] Building application...")
				build := exec.Command("go", "build", "-o", ".astra/tmp/server", ".")
				build.Stdout = os.Stdout
				build.Stderr = os.Stderr
				if err := build.Run(); err != nil {
					fmt.Println("[Astra Dev] Build failed! Waiting for further changes...")
					return
				}

				fmt.Println("[Astra Dev] Starting server...")
				process = exec.Command("./.astra/tmp/server")
				process.Stdout = os.Stdout
				process.Stderr = os.Stderr
				if err := process.Start(); err != nil {
					fmt.Println("[Astra Dev] Failed to start server:", err)
				}
			}

			if err := os.MkdirAll(".astra/tmp", 0750); err != nil {
				fmt.Printf("Failed to create tmp dir: %v\n", err)
			}

			// Initial Start
			startProcess()

			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						// Trigger on write or create of relevant files
						isRelevant := strings.HasSuffix(event.Name, ".go") ||
							strings.HasSuffix(event.Name, ".env") ||
							strings.HasSuffix(event.Name, ".html") ||
							strings.HasSuffix(event.Name, ".tmpl")

						if isRelevant && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
							mu.Lock()
							if buildCb != nil {
								buildCb.Stop()
							}
							buildCb = time.AfterFunc(400*time.Millisecond, func() {
								fmt.Printf("\n[Astra Dev] Changes detected in %s. Restarting...\n", filepath.Base(event.Name))
								startProcess()
							})
							mu.Unlock()
						}
					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						fmt.Println("Watcher error:", err)
					}
				}
			}()

			// Handle interrupt
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			<-quit

			mu.Lock()
			if process != nil && process.Process != nil {
				if err := process.Process.Signal(syscall.SIGTERM); err != nil {
					fmt.Printf("Failed to send SIGTERM: %v\n", err)
				}
				if err := process.Wait(); err != nil {
					fmt.Printf("Process wait error: %v\n", err)
				}
			}
			mu.Unlock()
		},
	}
}
