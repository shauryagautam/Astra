package controllers

import (
	"context"
	"fmt"

	"github.com/astraframework/astra/core"
	"github.com/astraframework/astra/http"
	"github.com/astraframework/astra/orm"
	"github.com/astraframework/astra/validate"
	"api_only/models" // We'll put Todo here
)

// TodoController handles API endpoints for todos.
type TodoController struct {
	app *core.App
}

func NewTodoController(app *core.App) *TodoController {
	return &TodoController{app: app}
}

// List returns all todos (GET /todos).
func (c *TodoController) List(ctx *http.Context) error {
	db := c.app.Get("db").(*orm.DB)
	
	// Fast, memory-safe slice scanning
	todos, err := orm.Query[models.Todo](db).Get(context.Background())
	if err != nil {
		return ctx.JSON(map[string]any{"error": err.Error()})
	}

	return ctx.JSON(map[string]any{"data": todos})
}

// Create payload structure
type createTodoReq struct {
	Title string `json:"title" validate:"required,min=3"`
}

// Create inserts a new todo (POST /todos).
func (c *TodoController) Create(ctx *http.Context) error {
	var req createTodoReq
	if err := ctx.Bind(&req); err != nil {
		return ctx.JSON(map[string]any{"error": "Invalid JSON"})
	}

	// Validate using the unified driver-agnostic validator
	v := c.app.Get("validator").(*validate.Validator)
	if err := v.ValidateStruct(req); err != nil {
		if ve, ok := err.(*validate.ValidationErrors); ok {
			return ctx.JSON(map[string]any{"errors": ve.Fields})
		}
		return ctx.JSON(map[string]any{"error": err.Error()})
	}

	db := c.app.Get("db").(*orm.DB)
	
	todo := models.Todo{
		Title: req.Title,
	}

	// Active Record style persistence
	if _, err := orm.Query[models.Todo](db).Create(todo, context.Background()); err != nil {
		return ctx.JSON(map[string]any{"error": fmt.Sprintf("failed to save: %v", err)})
	}

	return ctx.JSON(map[string]any{
		"message": "Todo created successfully",
		"data":    todo,
	})
}
