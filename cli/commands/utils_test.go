package commands

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCLIUtils(t *testing.T) {
	t.Run("toPascalCase", func(t *testing.T) {
		assert.Equal(t, "User", toPascalCase("user"))
		assert.Equal(t, "UserProfile", toPascalCase("user_profile"))
		assert.Equal(t, "UserProfile", toPascalCase("user-profile"))
	})

	t.Run("toSnakeCase", func(t *testing.T) {
		assert.Equal(t, "user_id", toSnakeCase("UserID"))
		assert.Equal(t, "name", toSnakeCase("Name"))
		assert.Equal(t, "first_name", toSnakeCase("FirstName"))
		assert.Equal(t, "user_profile", toSnakeCase("user_profile"))
	})

	t.Run("pluralize", func(t *testing.T) {
		assert.Equal(t, "users", pluralize("user"))
		assert.Equal(t, "categories", pluralize("category"))
		assert.Equal(t, "addresses", pluralize("address"))
	})
}
