package main

import (
	"context"
	"fmt"
	"log"
	"time"

	database "github.com/shauryagautam/Astra/pkg/database"
)

// User model
type User struct {
	ID        uint      `astra:"primaryKey"`
	Name      string    `astra:"size:255"`
	Email     string    `astra:"unique;size:255"`
	Active    bool      `astra:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Posts     database.HasMany[Post]
}

func (u User) TableName() string {
	return "users"
}

// Post model
type Post struct {
	ID        uint   `astra:"primaryKey"`
	UserID    uint   `astra:"index"`
	Title     string `astra:"size:255"`
	Body      string `astra:"type:text"`
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
