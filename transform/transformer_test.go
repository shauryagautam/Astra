package transform

import (
	"testing"

	"github.com/astraframework/astra/orm"
	"github.com/stretchr/testify/assert"
)

type user struct {
	ID    string
	Email string
}

type userTransformer struct{}

func (t *userTransformer) Transform(u user) map[string]any {
	return map[string]any{
		"id":    u.ID,
		"email": u.Email,
	}
}

func TestTransformer(t *testing.T) {
	trans := &userTransformer{}
	u1 := user{ID: "1", Email: "u1@example.com"}

	t.Run("Item", func(t *testing.T) {
		res := Item(u1, trans)
		assert.Equal(t, "1", res["id"])
		assert.Equal(t, "u1@example.com", res["email"])
	})

	t.Run("ItemP", func(t *testing.T) {
		res := ItemP(&u1, trans)
		assert.Equal(t, "1", res["id"])

		resNil := ItemP(nil, trans)
		assert.Nil(t, resNil)
	})

	t.Run("Collection", func(t *testing.T) {
		users := []user{u1, {ID: "2", Email: "u2@example.com"}}
		res := Collection(users, trans)
		assert.Len(t, res, 2)
		assert.Equal(t, "2", res[1]["id"])
	})

	t.Run("Paginated", func(t *testing.T) {
		p := orm.Paginated[user]{
			Data:     []user{u1},
			Total:    1,
			Page:     1,
			PerPage:  10,
			LastPage: 1,
		}
		res := Paginated(p, trans)
		data := res["data"].([]map[string]any)
		assert.Len(t, data, 1)
		assert.Equal(t, "1", data[0]["id"])
		assert.Equal(t, 1, res["total"])
	})

	t.Run("CursorPaginated", func(t *testing.T) {
		p := orm.CursorPaginated[user]{
			Data:       []user{u1},
			NextCursor: "next",
			HasMore:    true,
		}
		res := CursorPaginated(p, trans)
		data := res["data"].([]map[string]any)
		assert.Len(t, data, 1)
		assert.Equal(t, "next", res["next_cursor"])
		assert.True(t, res["has_more"].(bool))
	})

	t.Run("Func", func(t *testing.T) {
		fnTrans := Func(func(u user) map[string]any {
			return map[string]any{"mapped_id": u.ID}
		})
		res := Item(u1, fnTrans)
		assert.Equal(t, "1", res["mapped_id"])
	})
}
