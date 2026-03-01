package main

import (
	exceptions "github.com/shaurya/astra/app/Exceptions"
	validator "github.com/shaurya/astra/app/Validator"
	"github.com/shaurya/astra/contracts"
)

// UsersController handles user-related requests
type UsersController struct{}

// Index returns a list of users
func (u *UsersController) Index(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]any{
		"data": []map[string]any{
			{"id": 1, "name": "Alice", "email": "alice@example.com", "age": 25},
			{"id": 2, "name": "Bob", "email": "bob@example.com", "age": 30},
		},
	})
}

// Store creates a new user
func (u *UsersController) Store(ctx contracts.HttpContextContract) error {
	body := ctx.Request().All()

	v := validator.New()
	result := v.Validate(body, []contracts.FieldSchema{
		validator.String("name").Required().MinLength(2).MaxLength(100).Schema(),
		validator.String("email").Required().Email().Schema(),
		validator.Number("age").Min(0).Max(150).Schema(),
	})

	if result.HasErrors() {
		return exceptions.UnprocessableEntity("Validation failed", result.Errors)
	}

	return ctx.Response().Status(201).Json(map[string]any{
		"message": "User created successfully",
		"data":    body,
	})
}

// Show returns a single user
func (u *UsersController) Show(ctx contracts.HttpContextContract) error {
	id := ctx.Param("id")
	if id == "999" {
		return exceptions.NotFound("User not found")
	}

	return ctx.Response().Json(map[string]any{
		"id":    id,
		"name":  "Example User",
		"email": "user@example.com",
	})
}

// HealthController handles health checks
type HealthController struct{}

func (h *HealthController) Check(ctx contracts.HttpContextContract) error {
	return ctx.Response().Json(map[string]any{
		"status":  "ok",
		"uptime":  "healthy",
		"message": "Server is running 🚀",
	})
}
