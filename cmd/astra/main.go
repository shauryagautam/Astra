// Package main provides the Ace CLI — the command-line framework for Astra Go.
// Mirrors Astra's Ace CLI (node ace ...).
//
// Commands:
//
//	astra serve --watch         Start dev server with hot-reload
//	astra make:controller Name  Scaffold a controller
//	astra make:model Name       Scaffold a model
//	astra make:migration Name   Scaffold a migration
//	astra make:middleware Name  Scaffold a middleware
//	astra make:provider Name    Scaffold a provider
//	astra migration:run         Run pending migrations
//	astra migration:rollback    Rollback last batch
//	astra migration:status      Show migration status
//	astra db:seed               Run database seeders
//	astra routes:list           List all registered routes
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "astra",
	Short: "Astra Go — The Astra-inspired Go Framework CLI",
	Long: `
╔═══════════════════════════════════════════════════════════╗
║                     Astra Go CLI                         ║
║              ⚡ Powered by Go & Cobra ⚡                  ║
╚═══════════════════════════════════════════════════════════╝

The Ace CLI helps you scaffold, manage, and run your Astra Go application.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
