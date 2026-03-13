package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/graph-gophers/graphql-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rootResolver struct{}

func (r *rootResolver) Hello() string {
	return "world"
}

func (r *rootResolver) Echo(args struct{ Msg string }) string {
	return args.Msg
}

func (r *rootResolver) Now() DateTime {
	return DateTime{Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)}
}

func (r *rootResolver) ID() UUID {
	return UUID{UUID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}
}

func TestHandler(t *testing.T) {
	s := `
		scalar DateTime
		scalar UUID
		type Query {
			hello: String!
			echo(msg: String!): String!
			now: DateTime!
			id: UUID!
		}
	`
	schema := graphql.MustParseSchema(s, &rootResolver{})
	h := NewHandler(schema)

	t.Run("Basic Query", func(t *testing.T) {
		body := map[string]any{
			"query": "{ hello }",
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/graphql", bytes.NewReader(b))
		w := httptest.NewRecorder()

		h.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code)
		var resp struct {
			Data struct {
				Hello string `json:"hello"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "world", resp.Data.Hello)
	})

	t.Run("Ecno with Variables", func(t *testing.T) {
		body := map[string]any{
			"query": "query Echo($m: String!) { echo(msg: $m) }",
			"variables": map[string]any{
				"m": "hello astra",
			},
		}
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/graphql", bytes.NewReader(b))
		w := httptest.NewRecorder()

		h.ServeHTTP(w, req)

		var resp struct {
			Data struct {
				Echo string `json:"echo"`
			} `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "hello astra", resp.Data.Echo)
	})
}

func TestScalars(t *testing.T) {
	t.Run("DateTime", func(t *testing.T) {
		dt := &DateTime{}
		err := dt.UnmarshalGraphQL("2023-01-01T00:00:00Z")
		assert.NoError(t, err)
		assert.Equal(t, 2023, dt.Year())

		b, err := dt.MarshalJSON()
		assert.NoError(t, err)
		assert.Equal(t, "\"2023-01-01T00:00:00Z\"", string(b))
	})

	t.Run("UUID", func(t *testing.T) {
		u := &UUID{}
		idStr := "550e8400-e29b-41d4-a716-446655440000"
		err := u.UnmarshalGraphQL(idStr)
		assert.NoError(t, err)
		assert.Equal(t, idStr, u.String())

		b, err := u.MarshalJSON()
		assert.NoError(t, err)
		assert.Equal(t, "\""+idStr+"\"", string(b))
	})
}

func TestDataLoader(t *testing.T) {
	callCount := 0
	batchFn := func(ctx context.Context, keys []string) []*dataloader.Result[string] {
		callCount++
		results := make([]*dataloader.Result[string], len(keys))
		for i, key := range keys {
			results[i] = &dataloader.Result[string]{Data: "val-" + key}
		}
		return results
	}

	loader := NewDataLoader(batchFn)

	ctx := context.Background()
	thunk1 := loader.Load(ctx, "a")
	thunk2 := loader.Load(ctx, "b")

	val1, err1 := thunk1()
	val2, err2 := thunk2()

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, "val-a", val1)
	assert.Equal(t, "val-b", val2)
	assert.Equal(t, 1, callCount) // Should be batched
}
