package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/astraframework/astra/core"
	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

func ReplCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repl",
		Short: "Start an interactive Go REPL with Astra context",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Starting Astra REPL...")

			// Initialize the App in headless mode
			app, err := core.New()
			if err != nil {
				fmt.Printf("Error creating app: %v\n", err)
				return
			}

			// We don't call app.Start() because that starts the HTTP server and blocks.
			// Instead, we manually run the registration and boot phases.
			fmt.Println("Booting service providers...")

			// 1. Register
			for _, p := range app.Providers() {
				if err := p.Register(app); err != nil {
					fmt.Printf("Provider registration failed: %v\n", err)
					return
				}
			}

			// 2. Boot
			for _, p := range app.Providers() {
				if err := p.Boot(app); err != nil {
					fmt.Printf("Provider boot failed: %v\n", err)
					return
				}
			}

			// 3. Ready
			for _, p := range app.Providers() {
				if err := p.Ready(app); err != nil {
					fmt.Printf("Provider ready failed: %v\n", err)
					return
				}
			}

			fmt.Println("Astra REPL - Type 'exit' to quit, 'help()' for commands")

			i := interp.New(interp.Options{})
			if err := i.Use(stdlib.Symbols); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Expose 'app' to the interpreter
			if err := i.Use(interp.Exports{
				"astra/astra": {
					"App": reflect.ValueOf(app),
				},
			}); err != nil {
				fmt.Printf("Error: %v\n", err)
				return
			}

			// Pre-load common imports
			imports := []string{
				"import \"fmt\"",
				"import \"context\"",
				"import \"time\"",
				"import \"github.com/astraframework/astra/core\"",
				"import \"github.com/astraframework/astra/orm\"",
				"import \"github.com/astraframework/astra/http\"",
			}
			for _, imp := range imports {
				if _, err := i.Eval(imp); err != nil {
					fmt.Printf("Warning: failed to pre-load %s: %v\n", imp, err)
				}
			}

			// Define helper functions in the REPL
			_, _ = i.Eval(`
				func help() { 
					fmt.Println("Astra REPL Commands:")
					fmt.Println("  help()       - Show this help message")
					fmt.Println("  exit / quit  - Exit the REPL")
					fmt.Println("  clear()      - Clear the screen")
					fmt.Println("\nAvailable Context:")
					fmt.Println("  app          - The initialized Astra application instance (core.App)")
					fmt.Println("  ctx          - A background context (context.Context)")
					fmt.Println("  db           - The default ORM database connection")
				}
				var ctx = context.Background()
				var db = app.DB()

				func clear() {
					fmt.Print("\033[H\033[2J")
				}
			`)

			// Simple REPL loop with Readline
			rl, err := readline.NewEx(&readline.Config{
				Prompt:          "astra> ",
				HistoryFile:     filepath.Join(os.TempDir(), "astra_repl_history"),
				InterruptPrompt: "^C",
				EOFPrompt:       "exit",
			})
			if err != nil {
				fmt.Printf("Error initializing readline: %v\n", err)
				return
			}
			defer rl.Close()

			for {
				line, err := rl.Readline()
				if err != nil { // io.EOF or Interrupt
					break
				}
				input := strings.TrimSpace(line)

				if input == "exit" || input == "quit" {
					break
				}
				if input == "" {
					continue
				}

				res, err := i.Eval(input)
				if err != nil {
					fmt.Println("Error:", err)
					continue
				}
				if res.IsValid() {
					fmt.Println(res)
				}
			}

			// Shutdown app gracefully before exiting
			fmt.Println("\nShutting down app...")
			if err := app.Shutdown(5 * time.Second); err != nil {
				fmt.Printf("Shutdown error: %v\n", err)
			}
		},
	}
}
