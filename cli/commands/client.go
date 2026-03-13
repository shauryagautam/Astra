package commands

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/astraframework/astra/codegen"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

func ClientCmd() *cobra.Command {
	var watch bool

	cmd := &cobra.Command{
		Use:   "client",
		Short: "Generate TypeScript client",
		Run: func(cmd *cobra.Command, args []string) {
			generate := func() {
				fmt.Println("Generating TypeScript client...")
				err := codegen.GenerateClient(".", "shared/astra-client")
				if err != nil {
					fmt.Println("Failed to generate client:", err)
				} else {
					fmt.Println("TypeScript client generated in shared/astra-client")
				}
			}

			generate()

			if watch {
				fmt.Println("Watching for changes in app/ and routes/...")
				watcher, err := fsnotify.NewWatcher()
				if err != nil {
					log.Fatal(err)
				}
				defer watcher.Close()

				done := make(chan bool)
				go func() {
					lastGen := time.Now()
					for {
						select {
						case event, ok := <-watcher.Events:
							if !ok {
								return
							}
							if event.Op&fsnotify.Write == fsnotify.Write {
								// Debounce: wait at least 500ms between generations
								if time.Since(lastGen) > 500*time.Millisecond {
									fmt.Printf("File changed: %s, regenerating...\n", event.Name)
									generate()
									lastGen = time.Now()
								}
							}
						case err, ok := <-watcher.Errors:
							if !ok {
								return
							}
							fmt.Println("Watcher error:", err)
						}
					}
				}()

				// Add directories to watch
				dirsToWatch := []string{"app/models", "app/http/controllers", "routes"}
				for _, d := range dirsToWatch {
					if _, err := os.Stat(d); err == nil {
						_ = filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
							if err == nil && info.IsDir() {
								_ = watcher.Add(path)
							}
							return nil
						})
					}
				}

				<-done
			}
		},
	}

	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch for changes and regenerate client")

	return cmd
}
