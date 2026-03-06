package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// GraphqlCmd returns the `astra make:graphql-*` commands.
func GraphqlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graphql",
		Short: "GraphQL scaffolding commands",
	}

	cmd.AddCommand(makeResolverCmd())
	cmd.AddCommand(makeTypeCmd())

	return cmd
}

func makeResolverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:graphql-resolver [name]",
		Short: "Create a new GraphQL resolver",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := cases.Title(language.English).String(args[0])
			path := filepath.Join("app/graphql/resolvers", strings.ToLower(name)+".go")

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}

			content := fmt.Sprintf(`package resolvers

type %sResolver struct {
}

func (r *%sResolver) Query() string {
	return "Hello from %s"
}
`, name, name, name)

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}

			fmt.Printf("✓ Created resolver: %s\n", path)
			return nil
		},
	}
}

func makeTypeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:graphql-type [name]",
		Short: "Create a new GraphQL type definition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := cases.Title(language.English).String(args[0])
			path := filepath.Join("app/graphql/schema", strings.ToLower(name)+".graphql")

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}

			content := fmt.Sprintf(`type %s {
    id: ID!
    createdAt: String!
    updatedAt: String!
}

extend type Query {
    get%s(id: ID!): %s
}
`, name, name, name)

			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}

			fmt.Printf("✓ Created GraphQL schema: %s\n", path)
			return nil
		},
	}
}
