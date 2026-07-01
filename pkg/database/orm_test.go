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

func TestORM_SoftDeletePrecedence(t *testing.T) {
	db, err := Open(Config{Driver: "sqlite", DSN: ":memory:"})
	assert.NoError(t, err)
	defer db.Close()

	sqlStr, _ := Query[User](db).Where("name", "=", "Alice").OrWhere("email", "=", "bob@example.com").ToSQL()
	assert.Contains(t, sqlStr, "`deleted_at` IS NULL AND (`name` = ? OR `email` = ?)")
}

type SecureUser struct {
	Model
	Name string `orm:"column:name"`
	Role string `orm:"column:role;guarded"`
}

func (s *SecureUser) TableName() string {
	return "secure_users"
}

func TestORMSecurity(t *testing.T) {
	ctx := context.Background()
	db, err := Open(Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	assert.NoError(t, err)
	defer db.Close()

	// 1. Test SQL Injection via Operator validation
	t.Run("SQL Injection Operator Whitelist", func(t *testing.T) {
		q := Query[User](db).Where("name", "= OR 1=1; --", "Alice")
		_, err := q.Get(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsafe/invalid operator")
	})

	// 2. Test SQL Injection via Column name validation
	t.Run("SQL Injection Column Validation", func(t *testing.T) {
		q := Query[User](db).Where("name; DROP TABLE users; --", "=", "Alice")
		_, err := q.Get(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid column name")
	})

	// 3. Test SQL Injection via OrderBy direction validation
	t.Run("SQL Injection OrderBy Direction Validation", func(t *testing.T) {
		q := Query[User](db).OrderBy("name", "ASC; DROP TABLE users; --")
		_, err := q.Get(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid order direction")
	})

	// 4. Test Mass Assignment Protection on Update(data)
	t.Run("Mass Assignment Protection on Update", func(t *testing.T) {
		_, err = db.Exec(ctx, "CREATE TABLE secure_users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, role TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")
		assert.NoError(t, err)

		// Create user first
		su := SecureUser{Name: "Alice", Role: "user"}
		created, err := Query[SecureUser](db).Create(&su, ctx)
		assert.NoError(t, err)
		assert.Equal(t, "", created.Role) // role is guarded, so it is not inserted on Create

		// Attempt to update name and role via mass-update
		err = Query[SecureUser](db).Where("id", "=", created.ID).Update(map[string]any{
			"name": "Bob",
			"role": "admin",
		}, ctx)
		assert.NoError(t, err)

		// Verify that name was updated, but role remained "" (guarded)
		found, err := Query[SecureUser](db).Where("id", "=", created.ID).First(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "Bob", found.Name)
		assert.Equal(t, "", found.Role) // should NOT be admin
	})
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
