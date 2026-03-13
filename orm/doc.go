// Package orm provides a high-performance, type-safe Object-Relational Mapping (ORM) layer
// for the Astra framework. It is designed to be idiomatic Go, leveraging generics
// and reflection to provide a fluent API with minimal boilerplate.
//
// Key Features:
//   - Fluent Query Builder: orm.Query[Model](db)
//   - Relationships: HasOne, HasMany, BelongsTo, ManyToMany
//   - Eager Loading: Load relationships with .With("RelationName")
//   - Hooks: BeforeCreate, AfterSave, etc.
//   - Soft Deletes: Integrated soft-delete support via DeletedAt field.
//   - Transparent Encryption: Securely store PII with simple struct tags.
//   - Migrations: Versioned database schema management.
//
// Example usage:
//
//	// Define a model
//	type User struct {
//	    orm.Model
//	    Name  string `orm:"index"`
//	    Email string `orm:"unique"`
//	}
//
//	// Querying
//	users, err := orm.Query[User](db).Where("name", "ILIKE", "john%").Get(ctx)
//
//	// With Eager Loading
//	user, err := orm.Query[User](db).With("Posts").First(ctx)
package orm
