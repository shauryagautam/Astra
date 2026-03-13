package commands

import (
	"fmt"
	"strings"

	"github.com/astraframework/astra/config"
	"github.com/spf13/cobra"
)

// SecCmd returns the `astra sec` command.
func SecCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sec",
		Short: "Audit application security configuration",
		Long:  `Performs a security audit of your .env and configuration files to ensure secure defaults.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Astra Security Audit")
			fmt.Println("=====================")
			fmt.Println()

			env, _ := config.Load()
			cfg := config.LoadFromEnv(env)

			issues := 0

			// 1. APP_KEY
			if env.String("APP_KEY", "") == "" {
				fmt.Println("[✗] APP_KEY is not set. Security-critical features (Sessions, CSRF, Encrypted fields) will fail.")
				fmt.Println("    Fix: Run `astra key:generate`")
				issues++
			} else {
				fmt.Println("[✓] APP_KEY is set.")
			}

			// 2. APP_DEBUG
			if env.IsProd() && env.Bool("APP_DEBUG", false) {
				fmt.Println("[✗] APP_DEBUG is enabled in production. This may leak sensitive information in error pages.")
				fmt.Println("    Fix: Set APP_DEBUG=false in your production .env")
				issues++
			} else {
				fmt.Println("[✓] APP_DEBUG is safe for current environment.")
			}

			// 3. MaxBodySize
			if cfg.App.MaxBodySize <= 0 {
				fmt.Println("[!] MaxBodySize is not explicitly set or is disabled. Your app may be vulnerable to memory-exhaustion DoS.")
				fmt.Println("    Fix: Set APP_MAX_BODY_SIZE (in bytes) in your .env (e.g., 2097152 for 2MB)")
			} else {
				fmt.Printf("[✓] MaxBodySize is set to %d bytes.\n", cfg.App.MaxBodySize)
			}

			// 4. Session Security
			if env.IsProd() && !strings.HasPrefix(env.String("APP_URL", ""), "https://") {
				fmt.Println("[!] APP_URL is not HTTPS in production. Cookies may be intercepted if not using Secure flag.")
			}

			// 5. Database
			dbType := env.String("DB_CONNECTION", "postgres")
			if dbType == "sqlite" && env.IsProd() {
				fmt.Println("[!] SQLite is being used in production. Ensure the database file is not accessible via HTTP.")
			}

			fmt.Println()
			if issues == 0 {
				fmt.Println("No critical configuration issues found. Keep hardening!")
			} else {
				fmt.Printf("Audit complete. Found %d critical configuration issue(s).\n", issues)
			}
		},
	}
}
