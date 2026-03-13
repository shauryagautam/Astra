package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/astraframework/astra/orm"
	"github.com/astraframework/astra/orm/migration"
	"github.com/astraframework/astra/orm/schema"
)

// User model with all features
type User struct {
	orm.Model
	orm.Auditable

	Name     string                `orm:"column:name"`
	Email    string                `orm:"column:email"`
	Password orm.Encrypted[string] `orm:"column:password"`
	Role     string                `orm:"column:role"`
	Active   bool                  `orm:"column:active"`

	Posts   orm.HasMany[Post]   `orm:"foreignKey:user_id"`
	Profile orm.HasOne[Profile] `orm:"foreignKey:user_id"`
}

type Post struct {
	orm.Model

	Title   string `orm:"column:title"`
	Content string `orm:"column:content"`
	UserID  uint   `orm:"column:user_id"`

	User     orm.BelongsTo[User]  `orm:"foreignKey:user_id"`
	Comments orm.HasMany[Comment] `orm:"foreignKey:post_id"`
}

type Comment struct {
	orm.Model

	Body   string `orm:"column:body"`
	PostID uint   `orm:"column:post_id"`

	Post orm.BelongsTo[Post] `orm:"foreignKey:post_id"`
}

type Profile struct {
	orm.Model

	Bio    string `orm:"column:bio"`
	UserID uint   `orm:"column:user_id"`

	User orm.BelongsTo[User] `orm:"foreignKey:user_id"`
}

// Factories
var UserFactory = orm.Factory(func(f *orm.FactoryDef[User]) {
	f.Set("Name", orm.FakeName())
	f.Set("Email", orm.FakeEmail())
	f.Set("Password", orm.Encrypted[string]{Val: "secret123"})
	f.Set("Role", "user")
	f.Set("Active", true)
})

var PostFactory = orm.Factory(func(f *orm.FactoryDef[Post]) {
	f.Set("Title", orm.FakeText(50))
	f.Set("Content", orm.FakeText(200))
})

// Migrations
var CreateUsersMigration = &migration.SimpleMigration{
	Name: "20240101_create_users",
	UpFn: func(s *schema.Builder) error {
		return s.CreateTable("users", func(t *schema.Table) {
			t.ID()
			t.String("name", 255)
			t.String("email", 255).Unique()
			t.Text("password")
			t.String("role", 100).Default("user")
			t.Boolean("active").Default(true)
			t.Timestamps()
			t.SoftDeletes()
		})
	},
	DownFn: func(s *schema.Builder) error {
		return s.DropTable("users")
	},
}

var CreatePostsMigration = &migration.SimpleMigration{
	Name: "20240102_create_posts",
	UpFn: func(s *schema.Builder) error {
		return s.CreateTable("posts", func(t *schema.Table) {
			t.ID()
			t.String("title", 255)
			t.Text("content")
			t.Integer("user_id")
			t.Timestamps()
			t.SoftDeletes()
		})
	},
	DownFn: func(s *schema.Builder) error {
		return s.DropTable("posts")
	},
}

// Scopes
func AdminScope() func(*orm.QueryBuilder[User]) *orm.QueryBuilder[User] {
	return func(q *orm.QueryBuilder[User]) *orm.QueryBuilder[User] {
		return q.Where("role", "=", "admin")
	}
}

func ActiveScope() func(*orm.QueryBuilder[User]) *orm.QueryBuilder[User] {
	return func(q *orm.QueryBuilder[User]) *orm.QueryBuilder[User] {
		return q.Where("active", "=", true)
	}
}

func main() {
	ctx := context.Background()

	// Connect to database
	database, err := orm.Open(orm.Config{
		Driver:             "postgres",
		DSN:                "postgres://user:pass@localhost:5432/astra?sslmode=disable",
		MaxOpen:            25,
		MaxIdle:            5,
		Lifetime:           1 * time.Hour,
		SlowQueryThreshold: 100 * time.Millisecond,
		LogQueries:         true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	// Run migrations
	runner := migration.NewRunner(database, "")
	runner.Register("20240101_create_users", CreateUsersMigration)
	runner.Register("20240102_create_posts", CreatePostsMigration)
	if err := runner.Up(); err != nil {
		log.Fatal(err)
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// BASIC CRUD
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Create
	user := User{
		Name:     "John Doe",
		Email:    "john@example.com",
		Password: orm.Encrypted[string]{Val: "secret123"},
		Role:     "admin",
		Active:   true,
	}
	if _, err := orm.Query[User](database).Create(user, ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created user: %d\n", user.ID)

	// Find by ID
	found, err := orm.Query[User](database).Find(user.ID, ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found user: %s (%s)\n", found.Name, found.Email)

	// Update
	found.Name = "Jane Doe"
	if err := orm.Query[User](database).Save(found, ctx); err != nil {
		log.Fatal(err)
	}

	// Query with conditions
	users, err := orm.Query[User](database).
		Where("active", "=", true).
		Where("role", "=", "admin").
		OrderBy("created_at", "DESC").
		Limit(10).
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d active admins\n", len(users))

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// SCOPES
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	admins, err := orm.Query[User](database).
		Scope(AdminScope()).
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d active admins via scopes\n", len(admins))

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// RELATIONS
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Create related records
	post := Post{
		Title:   "My First Post",
		Content: "This is the content",
		UserID:  user.ID,
	}
	if _, err := orm.Query[Post](database).Create(post, ctx); err != nil {
		log.Fatal(err)
	}

	// Eager loading (N+1 prevention)
	usersWithPosts, err := orm.Query[User](database).
		With("Posts").
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, u := range usersWithPosts {
		fmt.Printf("User %s has %d posts\n", u.Name, len(u.Posts.Get()))
	}

	// Nested preloading
	postsWithComments, err := orm.Query[Post](database).
		With("User", "Comments").
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Loaded %d posts with users and comments\n", len(postsWithComments))

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// TRANSACTIONS
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	err = database.Transaction(ctx, func(tx *orm.DB) error {
		newUser := User{Name: "Alice", Email: "alice@example.com"}
		if _, err := orm.Query[User](tx).Create(newUser, ctx); err != nil {
			return err
		}

		newPost := Post{Title: "Alice's Post", UserID: 0} // ID will be set
		// or better, use the instance if we want to relate
		newPost.UserID = 1 // Simplified for example
		if _, err := orm.Query[Post](tx).Create(newPost, ctx); err != nil {
			return err
		}

		return nil // Commit
	})
	if err != nil {
		log.Fatal(err)
	}

	// Nested transactions (savepoints)
	err = database.Transaction(ctx, func(tx *orm.DB) error {
		user1 := User{Name: "Bob", Email: "bob@example.com"}
		if _, err := orm.Query[User](tx).Create(user1, ctx); err != nil {
			return err
		}

		return tx.Transaction(ctx, func(tx2 *orm.DB) error {
			user2 := User{Name: "Charlie", Email: "charlie@example.com"}
			_, err := orm.Query[User](tx2).Create(user2, ctx)
			return err
		})
	})
	if err != nil {
		log.Fatal(err)
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// PAGINATION
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	page, err := orm.Query[User](database).
		Where("active", "=", true).
		OrderBy("created_at", "DESC").
		Paginate(1, 10, ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Page %d/%d: %d users (total: %d)\n",
		page.CurrentPage, page.LastPage, len(page.Data), page.Total)

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// SOFT DELETE
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Soft delete
	if err := orm.Query[User](database).Where("id", "=", found.ID).Delete(ctx); err != nil {
		log.Fatal(err)
	}

	// Query excludes soft-deleted by default
	active, _ := orm.Query[User](database).Get(ctx)
	fmt.Printf("Active users: %d\n", len(active))

	// Include soft-deleted
	all, _ := orm.Query[User](database).WithTrashed().Get(ctx)
	fmt.Printf("All users (including deleted): %d\n", len(all))

	// Only soft-deleted
	// Note: orm doesn't have OnlyTrashed directly yet, using WhereNotNull("deleted_at")
	deleted, _ := orm.Query[User](database).WithTrashed().Where("deleted_at", "IS NOT NULL", nil).Get(ctx)
	fmt.Printf("Deleted users: %d\n", len(deleted))

	// Restore
	if err := orm.Query[User](database).Where("id", "=", found.ID).Restore(ctx); err != nil {
		log.Fatal(err)
	}

	// Force delete (permanent)
	if err := orm.Query[User](database).Where("id", "=", found.ID).ForceDelete(ctx); err != nil {
		log.Fatal(err)
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// FACTORIES & SEEDERS
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Create test data
	testUser, err := UserFactory.Create(ctx, database)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created test user: %s\n", testUser.Email)

	// Create many
	testUsers, err := UserFactory.CreateMany(ctx, 10, database)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created %d test users\n", len(testUsers))

	// With state
	admin, err := UserFactory.State("admin").Create(ctx, database)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created admin: %s\n", admin.Email)

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// RAW QUERIES
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	var rawUsers []User
	if err := database.Raw("SELECT * FROM users WHERE role = ? AND active = ?", "admin", true).Scan(&rawUsers, ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Raw query found %d admins\n", len(rawUsers))

	// Raw exec
	if _, err := database.Exec(ctx, "UPDATE users SET active = ? WHERE role = ?", false, "guest"); err != nil {
		log.Fatal(err)
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// ADVANCED QUERIES
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// WhereIn
	specificUsers, err := orm.Query[User](database).
		Where("id", "IN", []any{1, 2, 3}).
		Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d specific users\n", len(specificUsers))

	// Count
	count, err := orm.Query[User](database).
		Where("active", "=", true).
		Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Active user count: %d\n", count)

	// Exists
	exists, err := orm.Query[User](database).
		Where("email", "=", "john@example.com").
		Exists(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User exists: %v\n", exists)

	// Pluck
	emails, err := orm.Query[User](database).Pluck("email", ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("All emails: %v\n", emails)

	// FirstOrCreate
	newOrExisting, err := orm.Query[User](database).
		Where("email", "=", "unique@example.com").
		FirstOrCreate(User{
			Name:  "Unique User",
			Email: "unique@example.com",
		}, ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User: %s (ID: %d)\n", newOrExisting.Name, newOrExisting.ID)

	// Locking
	err = database.Transaction(ctx, func(tx *orm.DB) error {
		lockedUser, err := orm.Query[User](tx).
			Where("id", "=", 1).
			LockForUpdate().
			First(ctx)
		if err != nil {
			return err
		}

		lockedUser.Active = false
		return orm.Query[User](tx).Save(lockedUser, ctx)
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("✅ All ORM features demonstrated successfully!")
}
