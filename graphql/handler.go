package graphql

import (
	"encoding/json"
	"net/http"

	"github.com/graph-gophers/graphql-go"
)

// Handler implements http.Handler for executing GraphQL queries.
type Handler struct {
	Schema *graphql.Schema
}

// NewHandler creates a new GraphQL handler.
func NewHandler(schema *graphql.Schema) *Handler {
	return &Handler{Schema: schema}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var params struct {
		Query         string                 `json:"query"`
		OperationName string                 `json:"operationName"`
		Variables     map[string]interface{} `json:"variables"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	response := h.Schema.Exec(r.Context(), params.Query, params.OperationName, params.Variables)
	responseJSON, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to marshal response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(responseJSON); err != nil {
		// Ignore write error
	}
}
