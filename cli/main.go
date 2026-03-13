package main

import (
	"fmt"
	"os"

	"github.com/astraframework/astra/cli/commands"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "astra",
	Short: "Astra is a production-grade Go full-stack framework",
	Long:  `Astra provides everything you need to build web apps and mobile app backends with exceptional DX.`,
}

func main() {
	rootCmd.AddCommand(commands.NewCmd())
	rootCmd.AddCommand(commands.GenerateCmd())
	rootCmd.AddCommand(commands.ScaffoldCmd())
	rootCmd.AddCommand(commands.MakeCmd())
	commands.AddMakeAliases(rootCmd)
	commands.AddScaffoldAliases(rootCmd)
	rootCmd.AddCommand(commands.DbCmd())
	rootCmd.AddCommand(commands.MakeMigrationCmd())
	rootCmd.AddCommand(commands.DevCmd())
	rootCmd.AddCommand(commands.BuildCmd())
	rootCmd.AddCommand(commands.ClientCmd())
	rootCmd.AddCommand(commands.KeyGenerateCmd())
	rootCmd.AddCommand(commands.GraphqlCmd())
	rootCmd.AddCommand(commands.ReplCmd())
	rootCmd.AddCommand(commands.SecCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
