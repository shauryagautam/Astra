package database

// Standalone constructors allow using the Astra ORM in any Go project without
// pulling in the full framework framework. Add to your go.mod with:
//
//	go get github.com/shauryagautam/Astra/pkg/database
//
// Then use directly:
//
//	db, err := orm.NewStandalone(Config{
//	    Driver: "postgres",
//	    DSN:    os.Getenv("DATABASE_URL"),
//	})
//	defer db.Close()
//
//	type Post struct { ID int; Title string }
//	posts, err := database.NewQueryBuilder[Post](db).Get(ctx)

// NewStandalone establishes a database connection and returns an *DB.
// It is identical to Open() but explicitly documented as requiring no dependency
// on the engine.App framework struct. Suitable for:
//   - Standard net/http projects
//   - CLI tools that need ORM access
//   - Microservices that want only the ORM package
func NewStandalone(cfg Config) (*DB, error) {
	return Open(cfg)
}

// MustOpen calls Open and panics on error. Suitable for application startup
// where a database connection failure should be fatal.
func MustOpen(cfg Config) *DB {
	db, err := Open(cfg)
	if err != nil {
		panic("orm.MustOpen: " + err.Error())
	}
	return db
}
