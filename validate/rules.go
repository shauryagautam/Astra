package validate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5/pgxpool"
)

func existsRule(db *pgxpool.Pool) validator.Func {
	return func(fl validator.FieldLevel) bool {
		if db == nil {
			return true
		}
		param := fl.Param()
		parts := strings.Split(param, ".")
		if len(parts) != 2 {
			return false
		}
		table, col := parts[0], parts[1]
		val := fl.Field().Interface()

		// Use background context as a fallback if the validator doesn't provide one
		ctx := context.Background()

		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = $1)", table, col)
		var exists bool
		err := db.QueryRow(ctx, query, val).Scan(&exists)
		if err != nil {
			return false
		}
		return exists
	}
}

func uniqueRule(db *pgxpool.Pool) validator.Func {
	return func(fl validator.FieldLevel) bool {
		if db == nil {
			return true
		}
		param := fl.Param()
		parts := strings.Split(param, ":")
		tcParts := strings.Split(parts[0], ".")
		if len(tcParts) != 2 {
			return false
		}
		table, col := tcParts[0], tcParts[1]
		val := fl.Field().Interface()

		ctx := context.Background()

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
		err := db.QueryRow(ctx, query, args...).Scan(&count)
		if err != nil {
			return false
		}
		return count == 0
	}
}

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
