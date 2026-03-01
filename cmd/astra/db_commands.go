package main

import (
	"fmt"
	"os"

	"github.com/shaurya/astra/app"
	"github.com/shaurya/astra/contracts"
	"github.com/shaurya/astra/database"
	"github.com/shaurya/astra/providers"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func init() {
	rootCmd.AddCommand(migrationRunCmd)
	rootCmd.AddCommand(migrationRollbackCmd)
	rootCmd.AddCommand(migrationStatusCmd)
	rootCmd.AddCommand(migrationResetCmd)
	rootCmd.AddCommand(migrationRefreshCmd)
	rootCmd.AddCommand(dbSeedCmd)
}

// bootApp creates a minimal application with database provider for CLI commands.
func bootApp() (*app.Application, error) {
	application := app.NewApplication(".")

	application.RegisterProviders([]contracts.ServiceProviderContract{
		providers.NewAppProvider(application),
		providers.NewDatabaseProvider(application),
	})

	if err := application.Boot(); err != nil {
		return nil, fmt.Errorf("failed to boot application: %w", err)
	}

	return application, nil
}

// ══════════════════════════════════════════════════════════════════════
// migration:run
// ══════════════════════════════════════════════════════════════════════

var migrationRunCmd = &cobra.Command{
	Use:   "migration:run",
	Short: "Run pending migrations",
	Long:  `Executes all pending database migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		application, err := bootApp()
		if err != nil {
			return err
		}
		defer application.Shutdown() //nolint:errcheck

		runner := application.Use("Migration").(*database.MigrationRunner)

		// User should register their migrations here
		// runner.Add(migrations.CreateUsersTable, ...)

		return runner.Run()
	},
}

// ══════════════════════════════════════════════════════════════════════
// migration:rollback
// ══════════════════════════════════════════════════════════════════════

var migrationRollbackCmd = &cobra.Command{
	Use:   "migration:rollback",
	Short: "Rollback last batch of migrations",
	Long:  `Rolls back the most recent batch of migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		batch, _ := cmd.Flags().GetInt("batch")

		application, err := bootApp()
		if err != nil {
			return err
		}
		defer application.Shutdown() //nolint:errcheck

		runner := application.Use("Migration").(*database.MigrationRunner)

		if batch > 0 {
			return runner.RollbackBatch(batch)
		}
		return runner.Rollback()
	},
}

func init() {
	migrationRollbackCmd.Flags().Int("batch", 0, "Specific batch number to rollback (0 = latest)")
}

// ══════════════════════════════════════════════════════════════════════
// migration:status
// ══════════════════════════════════════════════════════════════════════

var migrationStatusCmd = &cobra.Command{
	Use:   "migration:status",
	Short: "Show migration status",
	Long:  `Displays the status of all registered migrations.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		application, err := bootApp()
		if err != nil {
			return err
		}
		defer application.Shutdown() //nolint:errcheck

		runner := application.Use("Migration").(*database.MigrationRunner)
		statuses, err := runner.Status()
		if err != nil {
			return err
		}

		fmt.Printf("%-50s %-10s %-8s %s\n", "MIGRATION", "STATUS", "BATCH", "APPLIED AT")
		fmt.Println(repeat("─", 90))
		for _, s := range statuses {
			appliedAt := ""
			if !s.AppliedAt.IsZero() {
				appliedAt = s.AppliedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%-50s %-10s %-8d %s\n", s.Name, s.Status, s.Batch, appliedAt)
		}

		return nil
	},
}

// ══════════════════════════════════════════════════════════════════════
// migration:reset
// ══════════════════════════════════════════════════════════════════════

var migrationResetCmd = &cobra.Command{
	Use:   "migration:reset",
	Short: "Rollback all migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		application, err := bootApp()
		if err != nil {
			return err
		}
		defer application.Shutdown() //nolint:errcheck

		runner := application.Use("Migration").(*database.MigrationRunner)
		return runner.Reset()
	},
}

// ══════════════════════════════════════════════════════════════════════
// migration:refresh
// ══════════════════════════════════════════════════════════════════════

var migrationRefreshCmd = &cobra.Command{
	Use:   "migration:refresh",
	Short: "Reset and re-run all migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		application, err := bootApp()
		if err != nil {
			return err
		}
		defer application.Shutdown() //nolint:errcheck

		runner := application.Use("Migration").(*database.MigrationRunner)
		return runner.Refresh()
	},
}

// ══════════════════════════════════════════════════════════════════════
// db:seed
// ══════════════════════════════════════════════════════════════════════

var dbSeedCmd = &cobra.Command{
	Use:   "db:seed",
	Short: "Run database seeders",
	Long:  `Executes all registered database seeders.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")

		application, err := bootApp()
		if err != nil {
			return err
		}
		defer application.Shutdown() //nolint:errcheck

		// Access DB for creating custom seeders
		_ = application.Use("Database").(*gorm.DB)
		seeder := application.Use("Seeder").(*database.SeederRunner)

		// User should register seeders here
		// seeder.Add(seeders.UserSeeder, seeders.PostSeeder)

		if name != "" {
			return seeder.RunByName(name)
		}
		return seeder.Run()
	},
}

func init() {
	dbSeedCmd.Flags().String("name", "", "Run a specific seeder by name")
}

// ══════════════════════════════════════════════════════════════════════
// routes:list
// ══════════════════════════════════════════════════════════════════════

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "routes:list",
		Short: "List all registered routes",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("📋 Routes are defined in start/routes.go")
			fmt.Println("   Run 'go run server.go' to see the route table at startup.")
			return nil
		},
	})
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// Suppress unused import warning
var _ = os.Stdout
