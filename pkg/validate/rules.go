package validate

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
)

// identRe matches valid SQL identifiers (letters, digits, underscores).
var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func sanitizeIdentifier(name string) (string, error) {
	if !identRe.MatchString(name) {
		return "", fmt.Errorf("invalid SQL identifier: %q", name)
	}
	return name, nil
}

// existsRule verifies that a value exists in table.column.
// Tag param syntax: validate:"exists=users.id"
func existsRule(db DBExecutor) validator.Func {
	return func(fl validator.FieldLevel) bool {
		param := fl.Param()
		parts := strings.Split(param, ".")
		if len(parts) != 2 {
			return false
		}
		table, col := parts[0], parts[1]
		val := fl.Field().Interface()

		table, err := sanitizeIdentifier(table)
		if err != nil {
			return false
		}
		col, err = sanitizeIdentifier(col)
		if err != nil {
			return false
		}

		// Portable EXISTS query (works on Postgres, MySQL, SQLite).
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", table, col)
		var count int
		if err := db.QueryRow(context.Background(), query, val).Scan(&count); err != nil {
			return false
		}
		return count > 0
	}
}

// uniqueRule verifies that a value does not already exist in table.column.
// Tag param syntax:
//
//	validate:"unique=users.email"                     // simple uniqueness
//	validate:"unique=users.email:ignore_id=42"        // exclude a specific row (useful for updates)
func uniqueRule(db DBExecutor) validator.Func {
	return func(fl validator.FieldLevel) bool {
		param := fl.Param()
		parts := strings.Split(param, ":")
		tcParts := strings.Split(parts[0], ".")
		if len(tcParts) != 2 {
			return false
		}
		table, col := tcParts[0], tcParts[1]
		val := fl.Field().Interface()

		table, err := sanitizeIdentifier(table)
		if err != nil {
			return false
		}
		col, err = sanitizeIdentifier(col)
		if err != nil {
			return false
		}

		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = $1", table, col)
		args := []any{val}

		if len(parts) > 1 {
			ignoreParts := strings.Split(parts[1], "=")
			if len(ignoreParts) == 2 && ignoreParts[0] == "ignore_id" {
				query += " AND id != $2"
				args = append(args, ignoreParts[1])
			}
		}

		var count int
		if err := db.QueryRow(context.Background(), query, args...).Scan(&count); err != nil {
			return false
		}
		return count == 0
	}
}

// afterDateRule validates that a time.Time field is after a given date.
// Tag param syntax:
//
//	validate:"after_date=now"             // must be in the future
//	validate:"after_date=2024-01-01"      // must be after specific date (YYYY-MM-DD)
func afterDateRule(fl validator.FieldLevel) bool {
	field, ok := fl.Field().Interface().(time.Time)
	if !ok {
		return false
	}
	param := fl.Param()

	var compareTo time.Time
	if param == "now" {
		compareTo = time.Now()
	} else {
		var err error
		compareTo, err = time.Parse("2006-01-02", param)
		if err != nil {
			compareTo, err = time.Parse(time.RFC3339, param)
			if err != nil {
				return false
			}
		}
	}

	return field.After(compareTo)
}
