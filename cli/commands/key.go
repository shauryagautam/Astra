package commands

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// KeyGenerateCmd returns the `astra key:generate` command.
func KeyGenerateCmd() *cobra.Command {
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "key:generate",
		Short: "Generate a new application key (APP_KEY)",
		Long: `Generates a cryptographically secure 32-byte random key,
base64-encodes it, and writes APP_KEY=base64:<key> to your .env file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Generate 32 random bytes
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				return fmt.Errorf("failed to generate random key: %w", err)
			}
			key := "base64:" + base64.StdEncoding.EncodeToString(raw)

			envFile := ".env"
			updated, err := updateEnvFile(envFile, "APP_KEY", key, forceFlag)
			if err != nil {
				return err
			}

			if updated {
				fmt.Printf("✓ APP_KEY updated in %s\n", envFile)
			} else {
				fmt.Printf("APP_KEY already set in %s. Use --force to overwrite.\n", envFile)
				fmt.Printf("Generated key (not written): %s\n", key)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing APP_KEY")
	return cmd
}

// updateEnvFile sets key=value in the .env file.
// Returns true if the file was updated, false if key already exists and force=false.
func updateEnvFile(filename, key, value string, force bool) (bool, error) {
	lines := []string{}
	found := false

	// Read existing .env
	f, err := os.Open(filename)
	if err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, key+"=") {
				if !force {
					f.Close()
					return false, nil
				}
				line = key + "=" + value
				found = true
			}
			lines = append(lines, line)
		}
		f.Close()
		if scanner.Err() != nil {
			return false, fmt.Errorf("failed to read %s: %w", filename, scanner.Err())
		}
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to open %s: %w", filename, err)
	}

	if !found {
		lines = append(lines, key+"="+value)
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
		return false, fmt.Errorf("failed to write %s: %w", filename, err)
	}
	return true, nil
}
