# 04. Persistence

Astra’s data layer is built for type safety, long-lived transactions, and real database behavior. The goal is not just fluent queries. The goal is to make the database layer predictable under load and composable in tests.

## Why the ORM is generic

`QueryBuilder[T]` lets the compiler carry the model type through the query chain. That removes the cast-heavy style common in older Go data layers and makes your repositories easier to read and harder to misuse.

The builder also keeps the API small. You compose filters, relationships, and limits with methods that return the same generic builder, then finish with a terminal call like `Get`, `All`, or `Chunk`.

---

## Defining Models & Relationships

Models are plain Go structs with optional `astra` tags to describe database behavior and relationships.

```go
type Post struct {
    ID        int64     `astra:"primary_key"`
    Title     string    `astra:"searchable"`
    Body      string
    UserID    int64     `astra:"foreign_key"`
    
    // Relationships
    Author    *User     `astra:"belongs_to"`
    Comments  []Comment `astra:"has_many"`
}
```

Astra supports:
- **`belongs_to`**: The current model owns the foreign key.
- **`has_one`**: The target model owns the foreign key.
- **`has_many`**: The target model owns a foreign key pointing back here.
- **`many_to_many`**: Uses a join table.

Use `.With("RelationshipName")` on the query builder to eager load these fields.

---

## ORM Lifecycle Hooks

Hooks tell the ORM to run logic before or after database operations. This is the right place for password hashing, UUID generation, or denormalization.

```go
func (u *User) BeforeSave(ctx context.Context) error {
    if u.IsNew() {
        u.ID = uuid.New()
    }
    return nil
}
```

Supported hooks:
- `BeforeSave`, `AfterSave`
- `BeforeCreate`, `AfterCreate`
- `BeforeUpdate`, `AfterUpdate`
- `BeforeDelete`, `AfterDelete`

---

## The Repository Pattern

While you can use `QueryBuilder[T]` directly in your handlers, Astra recommends keeping your data logic in Repositories. This makes your handlers easier to test and your queries reusable.

```go
type UserRepository struct {
    db *database.DB
}

func (r *UserRepository) FindActiveByEmail(ctx context.Context, email string) (*User, error) {
    return database.NewQueryBuilder[User](r.db).
        Where("email", "=", email).
        Where("status", "=", "active").
        First(ctx)
}
```

---

## Streaming rows with `iter.Seq2`

When you do not want to materialize everything into memory, `All()` returns an `iter.Seq2[*T, error]`. That is the Go 1.23+ shape for range-over-function iteration, which means you can process rows as they arrive.

```go
for user, err := range repo.FindAll(ctx) {
    if err != nil {
        return err
    }
    process(user)
}
```

---

## Nested transactions

Astra handles nested transactions with savepoints instead of faking nesting in application code.

The API is straightforward: call `db.Transaction(ctx, func(txCtx context.Context) error { ... })`. If you call `Transaction` again inside that block, Astra uses `SAVEPOINT` automatically.

---

## Copy-Paste Example

```go
package main

import (
    "context"
    "fmt"
    "github.com/shauryagautam/Astra/pkg/database"
)

type User struct {
    ID     int64
    Email  string
    Status string
}

func main() {
    db, _ := database.Open(database.Config{/* ... */})
    ctx := context.Background()

    // Using a repository
    repo := &UserRepository{db: db}
    user, err := repo.FindActiveByEmail(ctx, "hello@astra.dev")
    
    // Manual builder with eager loading
    posts, _ := database.NewQueryBuilder[Post](db).
        With("Author").
        Where("published", "=", true).
        Get(ctx)
}
```

---

**Next Chapter: [05. Frontend](./05-frontend.md)**
