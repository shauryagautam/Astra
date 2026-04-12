package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type user struct {
	ID    string
	Admin bool
}

type post struct {
	ID     string
	UserID string
}

func TestGate(t *testing.T) {
	g := New()

	g.Register("update", (*post)(nil), func(u, s any) bool {
		usr := u.(*user)
		pst, ok := s.(*post)
		if !ok {
			pVal := s.(post)
			pst = &pVal
		}
		return usr.Admin || usr.ID == pst.UserID
	})

	t.Run("Allows", func(t *testing.T) {
		u := &user{ID: "u1", Admin: false}
		p := &post{ID: "p1", UserID: "u1"}
		assert.True(t, g.Allows(u, "update", p))

		admin := &user{ID: "u2", Admin: true}
		assert.True(t, g.Allows(admin, "update", p))
	})

	t.Run("Denies", func(t *testing.T) {
		u := &user{ID: "u1", Admin: false}
		p := &post{ID: "p1", UserID: "u2"}
		assert.True(t, g.Denies(u, "update", p))
		assert.False(t, g.Allows(u, "update", p))
	})

	t.Run("Unregistered Policy", func(t *testing.T) {
		u := &user{ID: "u1"}
		p := &post{ID: "p1"}
		assert.False(t, g.Allows(u, "delete", p))
	})

	t.Run("Authorize", func(t *testing.T) {
		u := &user{ID: "u1", Admin: false}
		p := &post{ID: "p1", UserID: "u1"}
		assert.NoError(t, g.Authorize(u, "update", p))

		p2 := &post{ID: "p2", UserID: "u2"}
		err := g.Authorize(u, "update", p2)
		assert.Error(t, err)
		assert.Equal(t, 403, err.(*PolicyDeniedError).HTTPStatus())
	})

	t.Run("Type Matching", func(t *testing.T) {
		u := &user{ID: "u1"}
		p := post{ID: "p1", UserID: "u1"}
		// Should work because our PolicyFunc now handles both *post and post
		assert.True(t, g.Allows(u, "update", p))
	})
}

func TestRBAC(t *testing.T) {
	r := NewRBAC()

	r.DefineRole("admin", []string{"*"})
	r.DefineRole("editor", []string{"posts.*", "comments.view"})
	r.DefineRole("viewer", []string{"posts.view", "comments.view"})

	t.Run("Wildcard", func(t *testing.T) {
		assert.True(t, r.Can([]string{"admin"}, "anything"))
		assert.True(t, r.Can([]string{"editor"}, "posts.create"))
		assert.True(t, r.Can([]string{"editor"}, "posts.delete"))
		assert.False(t, r.Can([]string{"editor"}, "users.create"))
	})

	t.Run("Specific Permissions", func(t *testing.T) {
		assert.True(t, r.Can([]string{"viewer"}, "posts.view"))
		assert.False(t, r.Can([]string{"viewer"}, "posts.edit"))
	})

	t.Run("Multiple Roles", func(t *testing.T) {
		assert.True(t, r.Can([]string{"viewer", "editor"}, "posts.edit"))
		assert.False(t, r.Can([]string{"guest", "viewer"}, "posts.edit"))
	})

	t.Run("Non-existent Role", func(t *testing.T) {
		assert.False(t, r.Can([]string{"ghost"}, "posts.view"))
	})
}
