package main

import (
	"context"
	"fmt"
	"log"
	"time"

	database "github.com/shauryagautam/Astra/pkg/database"
	"github.com/shauryagautam/Astra/pkg/database/schema"
)

// User model
type User struct {
	ID        uint      `orm:"primaryKey;autoIncrement"`
	Name      string    `orm:"column:name"`
	Email     string    `orm:"column:email"`
	Active    bool      `orm:"column:active"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Posts     database.HasMany[Post]
}

func (u User) TableName() string {
	return "users"
}

// Post model
type Post struct {
	ID        uint   `orm:"primaryKey;autoIncrement"`
	UserID    uint   `orm:"column:user_id"`
	Title     string `orm:"column:title"`
	Body      string `orm:"column:body"`
	CreatedAt time.Time
	User      database.BelongsTo[User]
}

func main() {
	ctx := context.Background()

	// Initialize database
	cfg := database.Config{
		Driver: "sqlite3",
		DSN:    ":memory:",
	}
	db, err := database.Open(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fmt.Println("🚀 ORM Demo Running...")

	// Create tables
	err = db.Schema().CreateTableIfNotExists("users", func(t *schema.Table) {
		t.ID()
		t.String("name", 255)
		t.String("email", 255).Unique()
		t.Boolean("active").Default(true)
		t.Timestamps()
	})
	if err != nil {
		log.Fatal(err)
	}

	// Basic Create
	user := User{Name: "John Doe", Email: "john@example.com"}
	if _, err := database.NewQueryBuilder[User](db).Create(&user, ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created User: %s (ID: %d)\n", user.Name, user.ID)

	// Basic Find
	found, err := database.NewQueryBuilder[User](db).Where("email", "=", "john@example.com").First(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found User: %s\n", found.Name)

	fmt.Println("✅ ORM Demo finished successfully!")
}
