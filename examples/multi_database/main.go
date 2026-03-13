package main

import (
	"context"
	"fmt"
	"log"

	"github.com/astraframework/astra/orm"
)

type User struct {
	orm.Model
	Name  string `orm:"column:name"`
	Email string `orm:"column:email"`
}

func main() {
	ctx := context.Background()

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// POSTGRES — Primary transactional database
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Primary Database (Postgres)
	pg, err := orm.Open(orm.Config{
		Driver: "postgres",
		DSN:    "postgres://user:pass@localhost:5432/primary?sslmode=disable",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer pg.Close()

	// Standard ORM operations
	user := User{Name: "John Doe", Email: "john@example.com"}
	if _, err := orm.Query[User](pg).Create(user, ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✅ Created user in Postgres: %d\n", user.ID)

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// MYSQL — Secondary relational database
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Secondary Database (MySQL)
	my, err := orm.Open(orm.Config{
		Driver: "mysql",
		DSN:    "user:pass@tcp(localhost:3306)/secondary",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer my.Close()

	// Same ORM API works across databases
	if _, err := my.Exec(ctx,
		"UPDATE inventory SET stock = stock - 1 WHERE sku = ?",
		"IPHONE-15"); err != nil {
		log.Print(err) // Might fail if DB not setup, that's fine for example
	}
	fmt.Printf("✅ Executed raw query in MySQL\n")

	fmt.Println("✅ Multi-database support demonstrated successfully!")
}
