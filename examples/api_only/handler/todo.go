package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"api_only/schema"
	"github.com/shauryagautam/Astra/pkg/database"
	"github.com/shauryagautam/Astra/pkg/validate"
)

// ListTodoHandler handles GET /todos.
type ListTodoHandler struct {
	DB *database.DB
}

func (h *ListTodoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	todos, err := database.NewQueryBuilder[schema.Todo](h.DB).Get(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"data": todos})
}

// CreateTodoHandler handles POST /todos.
type CreateTodoHandler struct {
	DB        *database.DB
	Validator *validate.Validator
}

type createTodoReq struct {
	Title string `json:"title" validate:"required,min=3"`
}

func (h *CreateTodoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req createTodoReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "Invalid JSON"})
		return
	}

	if err := h.Validator.ValidateStruct(req); err != nil {
		if ve, ok := err.(*validate.ValidationErrors); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_ = json.NewEncoder(w).Encode(map[string]any{"errors": ve.Fields})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	todo := schema.Todo{
		Title: req.Title,
	}

	if _, err := database.NewQueryBuilder[schema.Todo](h.DB).Create(todo, r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf("failed to save: %v", err)})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message": "Todo created successfully",
		"data":    todo,
	})
}
