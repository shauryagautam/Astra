package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/database"
	"github.com/astraframework/astra/db"
	"github.com/astraframework/astra/migrations"
	"github.com/spf13/cobra"
)

// DbCmd returns the `astra db` command group.
func DbCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
	}

	cmd.AddCommand(dbMigrateCmd())
	cmd.AddCommand(dbRollbackCmd())
	cmd.AddCommand(dbStatusCmd())
	cmd.AddCommand(dbFreshCmd())
	cmd.AddCommand(dbSeedCmd())

	return cmd
}

func dbMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Run all pending database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}
			defer res.Pool.Close()

			fmt.Println("Running migrations...")
			return runner.Run(cmd.Context())
		},
	}
}

func dbRollbackCmd() *cobra.Command {
	var steps int
	rollback := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback the last N database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}
			defer res.Pool.Close()

			fmt.Printf("Rolling back last %d migration(s)...\n", steps)
			return runner.RollbackN(cmd.Context(), steps)
		},
	}
	rollback.Flags().IntVar(&steps, "step", 1, "Number of migrations to rollback")
	return rollback
}

func dbStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show migration status (applied vs pending)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}
			defer res.Pool.Close()

			applied, pending, err := runner.Status(cmd.Context())
			if err != nil {
				return err
			}

			fmt.Println("Migrations Status:")
			fmt.Printf("  %-40s %-10s %-20s\n", "Version", "Status", "Applied At")
			fmt.Println("  " + "----------------------------------------------------------------------")

			for _, rec := range applied {
				fmt.Printf("  %-40s %-10s %-20s\n", rec.Name, "✓ Applied", rec.RunAt.Format("2006-01-02 15:04:05"))
			}
			for _, p := range pending {
				fmt.Printf("  %-40s %-10s %-20s\n", p, "Pending", "-")
			}
			return nil
		},
	}
}

func dbFreshCmd() *cobra.Command {
	var confirm bool
	fresh := &cobra.Command{
		Use:   "fresh",
		Short: "Drop all tables and re-run all migrations (destructive — dev only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				fmt.Println("⚠️  This will DROP ALL TABLES. Use --force to confirm.")
				return nil
			}
			res, runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}
			defer res.Pool.Close()

			fmt.Println("Dropping all tables and re-running migrations...")
			return runner.Fresh(cmd.Context())
		},
	}
	fresh.Flags().BoolVar(&confirm, "force", false, "Confirm destructive fresh migration")
	return fresh
}

func dbSeedCmd() *cobra.Command {
	var name string
	seed := &cobra.Command{
		Use:   "seed",
		Short: "Run database seeders",
		RunE: func(cmd *cobra.Command, args []string) error {
			rawCfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg := config.LoadFromEnv(rawCfg)

			pool, err := db.Connect(cfg.Database)
			if err != nil {
				return err
			}
			defer pool.Close()

			runner := database.NewSeederRunner()
			// NOTE: Framework CLI cannot see user's registered seeders without reflection
			// or a global registry. For now, we'll assume the user uses the framework's logic.

			if name != "" {
				fmt.Printf("Running seeder: %s\n", name)
				return runner.RunByName(cmd.Context(), pool, name)
			}

			fmt.Println("Running all seeders...")
			return runner.Run(cmd.Context(), pool)
		},
	}
	seed.Flags().StringVar(&name, "name", "", "Run a specific seeder by name")
	return seed
}

func setupMigrationRunner() (*db.DB, *migrations.Runner, error) {
	rawCfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg := config.LoadFromEnv(rawCfg)

	pool, err := db.Connect(cfg.Database)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	dir := "database/migrations"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}

	runner := migrations.NewRunner(pool, dir, nil)
	return &db.DB{Pool: pool}, runner, nil
}

// MakeMigrationCmd returns a command to generate a new migration file.
func MakeMigrationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:migration [name]",
		Short: "Create a new migration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			timestamp := time.Now().Format("20060102150405")
			filename := fmt.Sprintf("%s_%s.sql", timestamp, name)
			dir := "database/migrations"

			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}

			path := filepath.Join(dir, filename)
			content := "-- +migrate Up\n\n-- +migrate Down\n"

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}

			fmt.Printf("✓ Created migration: %s\n", path)
			return nil
		},
	}
}
