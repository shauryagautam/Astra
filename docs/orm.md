# Database & ORM

Astra features a powerful Multi-Database ORM that provides a fluent API for interacting with your data while maintaining Go's type-safety.

## Defining Models

A model is a Go struct that embeds `orm.Model`.

```go
type User struct {
    orm.Model
    Email    string `orm:"unique;not null"`
    Name     string `orm:"index"`
    IsActive bool   `orm:"default:true"`
}
```

## Basic Queries

Use the fluent query builder for standard CRUD operations.

```go
// Find by ID
user := &User{}
err := orm.Find(user, 1)

// Query with constraints
var users []User
err := orm.Where("is_active = ?", true).
           Order("created_at DESC").
           Limit(10).
           Find(&users)
```

## Creating & Updating

```go
// Create
user := &User{Email: "test@example.com", Name: "Test"}
err := orm.Create(user)

// Update
user.Name = "New Name"
err := orm.Save(user)
```

## Relationships

Astra supports all common relationship types: `HasOne`, `HasMany`, `BelongsTo`, and `ManyToMany`.

```go
type Post struct {
    orm.Model
    Title   string
    UserID  uint
    User    orm.BelongsTo[User]
}

// Eager Loading
var posts []Post
err := orm.With("User").Find(&posts)
```

## Migrations

Migrations are Go files that define changes to your database schema over time.

### Create a Migration
```bash
astra make:migration create_users_table
```

### Run Migrations
```bash
astra migrate:up
```

## Transparent Encryption

Secure sensitive data (like PII) with zero effort by using the `encrypted` tag.

```go
type User struct {
    orm.Model
    SSN string `orm:"encrypted"` // Automatically encrypted/decrypted at rest
}
```
