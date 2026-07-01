package database

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Define relation test models
type Post struct {
	Model
	Title    string `orm:"column:title"`
	Comments HasMany[Comment]
}

func (p *Post) TableName() string { return "posts" }

type Comment struct {
	Model
	Body   string `orm:"column:body"`
	PostID uint   `orm:"column:post_id"`
	Post   BelongsTo[Post]
}

func (c *Comment) TableName() string { return "comments" }

type Profile struct {
	Model
	Bio    string `orm:"column:bio"`
	UserID uint   `orm:"column:user_id"`
}

func (pr *Profile) TableName() string { return "profiles" }

type RelUser struct {
	Model
	Name    string `orm:"column:name"`
	Profile HasOne[Profile] `orm:"foreignKey:user_id"`
	Roles   ManyToMany[Role]
}

func (u *RelUser) TableName() string { return "rel_users" }

type Role struct {
	Model
	Name string `orm:"column:name"`
}

func (r *Role) TableName() string { return "roles" }

// Polymorphic MorphTo Models
type Image struct {
	Model
	URL           string  `orm:"column:url"`
	ImageableType string  `orm:"column:imageable_type"`
	ImageableID   uint    `orm:"column:imageable_id"`
	Imageable     MorphTo `orm:"morphTo;morphType:imageable_type;morphID:imageable_id"`
}

func (i *Image) TableName() string { return "images" }

func TestORMRelations(t *testing.T) {
	ctx := context.Background()
	db, err := Open(Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	assert.NoError(t, err)
	defer db.Close()

	// Warm up model registry
	importReflect := reflect.TypeOf(Post{})
	GetMeta(importReflect)
	GetMeta(reflect.TypeOf(Comment{}))
	GetMeta(reflect.TypeOf(Profile{}))
	GetMeta(reflect.TypeOf(RelUser{}))
	GetMeta(reflect.TypeOf(Role{}))
	GetMeta(reflect.TypeOf(Image{}))

	// Create tables
	statements := []string{
		"CREATE TABLE posts (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)",
		"CREATE TABLE comments (id INTEGER PRIMARY KEY AUTOINCREMENT, body TEXT, post_id INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)",
		"CREATE TABLE profiles (id INTEGER PRIMARY KEY AUTOINCREMENT, bio TEXT, user_id INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)",
		"CREATE TABLE rel_users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)",
		"CREATE TABLE roles (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)",
		"CREATE TABLE rel_users_roles (rel_user_id INTEGER, role_id INTEGER)",
		"CREATE TABLE images (id INTEGER PRIMARY KEY AUTOINCREMENT, url TEXT, imageable_type TEXT, imageable_id INTEGER, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)",
	}

	for _, stmt := range statements {
		_, err = db.Exec(ctx, stmt)
		assert.NoError(t, err)
	}

	// Insert Test Data
	post, err := Query[Post](db).Create(&Post{Title: "Astra Guide"}, ctx)
	assert.NoError(t, err)
	_, err = Query[Comment](db).Create(&Comment{Body: "Great post!", PostID: post.ID}, ctx)
	assert.NoError(t, err)
	_, err = Query[Comment](db).Create(&Comment{Body: "Awesome!", PostID: post.ID}, ctx)
	assert.NoError(t, err)

	user, _ := Query[RelUser](db).Create(&RelUser{Name: "Alice"}, ctx)
	_, _ = Query[Profile](db).Create(&Profile{Bio: "Astra Developer", UserID: user.ID}, ctx)

	role1, _ := Query[Role](db).Create(&Role{Name: "Admin"}, ctx)
	role2, _ := Query[Role](db).Create(&Role{Name: "Editor"}, ctx)
	_, _ = db.Exec(ctx, "INSERT INTO rel_users_roles (rel_user_id, role_id) VALUES (?, ?)", user.ID, role1.ID)
	_, _ = db.Exec(ctx, "INSERT INTO rel_users_roles (rel_user_id, role_id) VALUES (?, ?)", user.ID, role2.ID)

	// Insert Polymorphic Data
	_, _ = Query[Image](db).Create(&Image{URL: "post_img.png", ImageableType: "Post", ImageableID: post.ID}, ctx)
	_, _ = Query[Image](db).Create(&Image{URL: "user_img.png", ImageableType: "RelUser", ImageableID: user.ID}, ctx)

	// Test HasMany
	t.Run("HasMany Eager Loading", func(t *testing.T) {
		fetchedPost, err := Query[Post](db).With("Comments").Where("id", "=", post.ID).First(ctx)
		assert.NoError(t, err)
		assert.True(t, fetchedPost.Comments.Loaded)
		assert.Len(t, fetchedPost.Comments.All(), 2)
		assert.Equal(t, "Great post!", fetchedPost.Comments.All()[0].Body)
	})

	// Test BelongsTo
	t.Run("BelongsTo Eager Loading", func(t *testing.T) {
		fetchedComment, err := Query[Comment](db).With("Post").Where("post_id", "=", post.ID).First(ctx)
		assert.NoError(t, err)
		assert.True(t, fetchedComment.Post.Loaded)
		assert.NotNil(t, fetchedComment.Post.Get())
		assert.Equal(t, "Astra Guide", fetchedComment.Post.Get().Title)
	})

	// Test HasOne
	t.Run("HasOne Eager Loading", func(t *testing.T) {
		fetchedUser, err := Query[RelUser](db).With("Profile").Where("id", "=", user.ID).First(ctx)
		assert.NoError(t, err)
		assert.True(t, fetchedUser.Profile.Loaded)
		assert.NotNil(t, fetchedUser.Profile.Get())
		assert.Equal(t, "Astra Developer", fetchedUser.Profile.Get().Bio)
	})

	// Test ManyToMany
	t.Run("ManyToMany Eager Loading", func(t *testing.T) {
		fetchedUser, err := Query[RelUser](db).With("Roles").Where("id", "=", user.ID).First(ctx)
		assert.NoError(t, err)
		assert.True(t, fetchedUser.Roles.Loaded)
		assert.Len(t, fetchedUser.Roles.All(), 2)
		assert.Equal(t, "Admin", fetchedUser.Roles.All()[0].Name)
	})

	// Test MorphTo
	t.Run("MorphTo Eager Loading", func(t *testing.T) {
		images, err := Query[Image](db).With("Imageable").Get(ctx)
		assert.NoError(t, err)
		assert.Len(t, images, 2)

		// First image belongs to Post
		img1 := images[0]
		assert.Equal(t, "post_img.png", img1.URL)
		assert.True(t, img1.Imageable.Loaded)
		imgPost, ok := img1.Imageable.Get().(*Post)
		assert.True(t, ok)
		assert.Equal(t, "Astra Guide", imgPost.Title)

		// Second image belongs to RelUser
		img2 := images[1]
		assert.Equal(t, "user_img.png", img2.URL)
		assert.True(t, img2.Imageable.Loaded)
		imgUser, ok := img2.Imageable.Get().(*RelUser)
		assert.True(t, ok)
		assert.Equal(t, "Alice", imgUser.Name)
	})
}
