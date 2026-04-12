package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type User struct {
	Model
	Name  string `orm:"column:name"`
	Email string `orm:"column:email"`
}

func (u *User) TableName() string {
	return "users"
}

func TestORM(t *testing.T) {
	ctx := context.Background()
	db, err := Open(Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	assert.NoError(t, err)
	defer db.Close()

	// Create table
	_, err = db.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")
	assert.NoError(t, err)

	// Test Create
	user := User{Name: "Alice", Email: "alice@example.com"}
	created, err := Query[User](db).Create(&user, ctx)
	assert.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.NotZero(t, created.CreatedAt)

	// Test Query
	found, err := Query[User](db).Where("name", "=", "Alice").First(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "Alice", found.Name)
	assert.Equal(t, "alice@example.com", found.Email)

	// Test Update (Save)
	found.Name = "Bob"
	err = Query[User](db).Save(found, ctx)
	assert.NoError(t, err)

	updated, err := Query[User](db).Where("id", "=", found.ID).First(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "Bob", updated.Name)

	// Test Delete (soft delete via Where + Delete)
	err = Query[User](db).Where("id", "=", updated.ID).Delete(ctx)
	assert.NoError(t, err)

	// Should not find (soft deleted)
	_, err = Query[User](db).Where("id", "=", found.ID).First(ctx)
	assert.Error(t, err)

	// Should find with trashed
	trashed, err := Query[User](db).WithTrashed().Where("id", "=", found.ID).First(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, trashed.DeletedAt)
}

func TestORM_Iterators(t *testing.T) {
	ctx := context.Background()
	db, _ := Open(Config{Driver: "sqlite", DSN: ":memory:"})
	_, _ = db.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")

	// Insert test data
	for i := 1; i <= 10; i++ {
		user := User{Name: "User", Email: "user@example.com"}
		_, _ = Query[User](db).Create(&user, ctx)
	}

	// Test All (iterator)
	t.Run("All", func(t *testing.T) {
		count := 0
		for user, err := range Query[User](db).All(ctx) {
			assert.NoError(t, err)
			assert.NotNil(t, user)
			count++
		}
		assert.Equal(t, 10, count)
	})

	// Test Each
	t.Run("Each", func(t *testing.T) {
		count := 0
		err := Query[User](db).Each(func(u *User) error {
			count++
			return nil
		}, ctx)
		assert.NoError(t, err)
		assert.Equal(t, 10, count)
	})

	// Test Chunk
	t.Run("Chunk", func(t *testing.T) {
		chunkCount := 0
		totalCount := 0
		err := Query[User](db).Chunk(3, func(users []User) error {
			chunkCount++
			totalCount += len(users)
			return nil
		}, ctx)
		assert.NoError(t, err)
		assert.Equal(t, 4, chunkCount) // 3, 3, 3, 1
		assert.Equal(t, 10, totalCount)
	})

	// Test RawQuery All
	t.Run("Raw_All", func(t *testing.T) {
		count := 0
		for user, err := range Raw[User](db, "SELECT * FROM users").All(ctx) {
			assert.NoError(t, err)
			assert.NotEmpty(t, user.Name)
			count++
		}
		assert.Equal(t, 10, count)
	})
}

func TestRawQuery(t *testing.T) {
	ctx := context.Background()
	db, err := Open(Config{Driver: "sqlite", DSN: ":memory:"})
	assert.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")
	assert.NoError(t, err)

	// Insert a user
	user := User{Name: "RawTest", Email: "raw@example.com"}
	_, err = Query[User](db).Create(&user, ctx)
	assert.NoError(t, err)

	// Test RawQuery.Scan
	var users []User
	err = Raw[User](db, "SELECT * FROM users WHERE name = ?", "RawTest").Scan(&users, ctx)
	assert.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, "RawTest", users[0].Name)
}

func BenchmarkScanner(b *testing.B) {
	ctx := context.Background()
	db, _ := Open(Config{Driver: "sqlite", DSN: ":memory:"})
	db.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")

	for i := 0; i < 1000; i++ {
		user := User{Name: "User", Email: "user@example.com"}
		Query[User](db).Create(&user, ctx)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Query[User](db).Get(ctx)
	}
}
