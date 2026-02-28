// Package main provides the Ace CLI — the command-line framework for Adonis Go.
// Mirrors AdonisJS's Ace CLI (node ace ...).
//
// Commands:
//
//	adonis serve --watch         Start dev server with hot-reload
//	adonis make:controller Name  Scaffold a controller
//	adonis make:model Name       Scaffold a model
//	adonis make:migration Name   Scaffold a migration
//	adonis make:middleware Name  Scaffold a middleware
//	adonis make:provider Name    Scaffold a provider
//	adonis migration:run         Run pending migrations
//	adonis migration:rollback    Rollback last batch
//	adonis migration:status      Show migration status
//	adonis db:seed               Run database seeders
//	adonis routes:list           List all registered routes
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "adonis",
	Short: "Adonis Go — The AdonisJS-inspired Go Framework CLI",
	Long: `
╔═══════════════════════════════════════════════════════════╗
║                     Adonis Go CLI                         ║
║              ⚡ Powered by Go & Cobra ⚡                  ║
╚═══════════════════════════════════════════════════════════╝

The Ace CLI helps you scaffold, manage, and run your Adonis Go application.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
