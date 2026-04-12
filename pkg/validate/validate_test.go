package validate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestUser struct {
	Name     string    `validate:"required,min=3"`
	Email    string    `validate:"required,email"`
	Birthday time.Time `validate:"required,after_date=1900-01-01"`
}

func TestValidator(t *testing.T) {
	v := New()

	t.Run("Valid Struct", func(t *testing.T) {
		user := TestUser{
			Name:     "Astra",
			Email:    "test@example.com",
			Birthday: time.Now(),
		}
		err := v.ValidateStruct(user)
		assert.NoError(t, err)
	})

	t.Run("Invalid Struct", func(t *testing.T) {
		user := TestUser{
			Name:     "As", // too short
			Email:    "invalid-email",
			Birthday: time.Date(1800, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		err := v.ValidateStruct(user)
		assert.Error(t, err)

		ve, ok := err.(*ValidationErrors)
		require.True(t, ok)
		assert.Len(t, ve.Fields, 3)
		assert.Contains(t, ve.Fields["name"][0], "at least 3 characters")
		assert.Contains(t, ve.Fields["email"][0], "valid email address")
		assert.Contains(t, ve.Fields["birthday"][0], "must be after")
	})
}

func TestAfterDateRule(t *testing.T) {
	v := New()

	type DateOnly struct {
		Date time.Time `validate:"after_date=now"`
	}

	t.Run("After Now", func(t *testing.T) {
		d := DateOnly{Date: time.Now().Add(24 * time.Hour)}
		assert.NoError(t, v.ValidateStruct(d))
	})

	t.Run("Before Now", func(t *testing.T) {
		d := DateOnly{Date: time.Now().Add(-24 * time.Hour)}
		assert.Error(t, v.ValidateStruct(d))
	})
}

func TestToSnakeCase(t *testing.T) {
	assert.Equal(t, "user_id", toSnakeCase("UserID"))
	assert.Equal(t, "name", toSnakeCase("Name"))
	assert.Equal(t, "first_name", toSnakeCase("FirstName"))
}
