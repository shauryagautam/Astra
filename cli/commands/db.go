package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/astraframework/astra/database"
	"github.com/astraframework/astra/orm"
	"github.com/astraframework/astra/orm/migration"
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
			runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}

			fmt.Println("Running migrations...")
			return runner.Up()
		},
	}
}

func dbRollbackCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rollback",
		Short: "Rollback the last batch of database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}

			fmt.Println("Rolling back last batch of migrations...")
			return runner.Rollback()
		},
	}
}

func dbStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the status of each migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}

			return runner.Status()
		},
	}
}

func dbFreshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fresh",
		Short: "Drop all tables and re-run all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}

			fmt.Println("Dropping all tables...")
			if err := runner.DB().DropAllTables(cmd.Context()); err != nil {
				return err
			}

			fmt.Println("Running migrations...")
			if err := runner.Up(); err != nil {
				return err
			}

			seed, _ := cmd.Flags().GetBool("seed")
			if seed {
				return runSeed(cmd.Context(), runner.DB())
			}
			return nil
		},
	}
	cmd.Flags().Bool("seed", false, "Seed the database after fresh reset")
	return cmd
}

func dbSeedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "seed",
		Short: "Run database seeders",
		RunE: func(cmd *cobra.Command, args []string) error {
			runner, err := setupMigrationRunner()
			if err != nil {
				return err
			}
			return runSeed(cmd.Context(), runner.DB())
		},
	}
}

func runSeed(_ context.Context, db *orm.DB) error {
	fmt.Println("Running database seeders...")
	return database.DefaultRunner.Run(context.Background(), db)
}

func setupMigrationRunner() (*migration.Runner, error) {
	rawCfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	cfg := config.LoadFromEnv(rawCfg)

	ormCfg := orm.Config{
		Driver: rawCfg.String("DB_DRIVER", "postgres"),
		DSN:    cfg.Database.URL,
	}

	d, err := orm.Open(ormCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	dir := "database/migrations"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	runner := migration.NewRunner(d, dir)
	return runner, nil
}

// MakeMigrationCmd returns a command to generate a new migration file.
func MakeMigrationCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:migration [name]",
		Short: "Create a new migration file (Go-based for new ORM)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			timestamp := time.Now().Format("20060102150405")
			filename := fmt.Sprintf("%s_%s.go", timestamp, name)
			dir := "database/migrations"

			if err := os.MkdirAll(dir, 0750); err != nil {
				return err
			}

			path := filepath.Join(dir, filename)
			content := fmt.Sprintf(`package migrations

import "github.com/astraframework/astra/orm/schema"

type Migration_%s struct{}

func (m *Migration_%s) Up(s *schema.Builder) error {
	return s.CreateTable("%s", func(t *schema.Table) {
		t.ID()
		t.Timestamps()
	})
}

func (m *Migration_%s) Down(s *schema.Builder) error {
	return s.DropTable("%s")
}
`, timestamp, timestamp, name, timestamp, name)

			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				return err
			}

			fmt.Printf("✓ Created Go migration: %s\n", path)
			return nil
		},
	}
}
