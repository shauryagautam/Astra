package dto

import (
	"testing"

	"github.com/shauryagautam/Astra/pkg/database"
	"github.com/stretchr/testify/assert"
)

type user struct {
	ID    string
	Email string
}

type userMapper struct{}

func (m *userMapper) ToDTO(u user) map[string]any {
	return map[string]any{
		"id":    u.ID,
		"email": u.Email,
	}
}

func TestMapper(t *testing.T) {
	mapper := &userMapper{}
	u1 := user{ID: "1", Email: "u1@example.com"}

	t.Run("MapItem", func(t *testing.T) {
		res := MapItem(u1, mapper)
		assert.Equal(t, "1", res["id"])
		assert.Equal(t, "u1@example.com", res["email"])
	})

	t.Run("MapItemP", func(t *testing.T) {
		res := MapItemP(&u1, mapper)
		assert.Equal(t, "1", res["id"])

		resNil := MapItemP(nil, mapper)
		assert.Nil(t, resNil)
	})

	t.Run("MapCollection", func(t *testing.T) {
		users := []user{u1, {ID: "2", Email: "u2@example.com"}}
		res := MapCollection(users, mapper)
		assert.Len(t, res, 2)
		assert.Equal(t, "2", res[1]["id"])
	})

	t.Run("MapPaginated", func(t *testing.T) {
		p := database.PaginationResult[user]{
			Data:        []user{u1},
			Total:       1,
			CurrentPage: 1,
			PerPage:     10,
			LastPage:    1,
		}
		res := MapPaginated(p, mapper)
		data := res["data"].([]map[string]any)
		assert.Len(t, data, 1)
		assert.Equal(t, "1", data[0]["id"])
		assert.Equal(t, int64(1), res["total"])
	})

	t.Run("MapCursorPaginated", func(t *testing.T) {
		p := database.CursorPaginated[user]{
			Data:       []user{u1},
			NextCursor: "next",
			HasMore:    true,
		}
		res := MapCursorPaginated(p, mapper)
		data := res["data"].([]map[string]any)
		assert.Len(t, data, 1)
		assert.Equal(t, "next", res["next_cursor"])
		assert.True(t, res["has_more"].(bool))
	})

	t.Run("MapFunc", func(t *testing.T) {
		fnMapper := MapFunc(func(u user) map[string]any {
			return map[string]any{"mapped_id": u.ID}
		})
		res := MapItem(u1, fnMapper)
		assert.Equal(t, "1", res["mapped_id"])
	})
}
