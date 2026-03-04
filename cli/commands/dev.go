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
			err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
				if info != nil && info.IsDir() && !strings.HasPrefix(info.Name(), ".") && !strings.HasPrefix(path, ".") {
					return watcher.Add(path)
				}
				return nil
			})
			if err != nil {
				fmt.Println("Failed to watch directories:", err)
				return
			}

			// Also watch current dir manually just in case
			watcher.Add(".")

			var (
				mu      sync.Mutex
				process *exec.Cmd
				buildCb *time.Timer
			)

			startProcess := func() {
				mu.Lock()
				defer mu.Unlock()

				if process != nil && process.Process != nil {
					process.Process.Kill()
					process.Wait()
				}

				fmt.Println("\nBuilding...")
				build := exec.Command("go", "build", "-o", ".astra/tmp/server", ".")
				build.Stdout = os.Stdout
				build.Stderr = os.Stderr
				if err := build.Run(); err != nil {
					fmt.Println("Build failed! Waiting for changes...")
					return
				}

				fmt.Println("Starting...")
				process = exec.Command("./.astra/tmp/server")
				process.Stdout = os.Stdout
				process.Stderr = os.Stderr
				if err := process.Start(); err != nil {
					fmt.Println("Failed to start server:", err)
				}
			}

			// Ensure target dir exists
			os.MkdirAll(".astra/tmp", 0755)

			// Initial Start
			startProcess()

			go func() {
				for {
					select {
					case event, ok := <-watcher.Events:
						if !ok {
							return
						}
						// Only trigger on write or create of .go files
						if strings.HasSuffix(event.Name, ".go") && (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) {
							mu.Lock()
							if buildCb != nil {
								buildCb.Stop()
							}
							buildCb = time.AfterFunc(300*time.Millisecond, func() {
								fmt.Printf("→ Restarting due to changes in %s\n", event.Name)
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
				process.Process.Kill()
				process.Wait()
			}
			mu.Unlock()
		},
	}
}
