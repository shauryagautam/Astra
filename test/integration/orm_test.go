package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/shauryagautam/Astra/pkg/database"
	astratesting "github.com/shauryagautam/Astra/pkg/test_util"
	"github.com/stretchr/testify/suite"
)

type ORMIntegrationSuite struct {
	suite.Suite
	Astra astratesting.Suite
}

type User struct {
	database.Model
	Email string `orm:"unique"`
	Name  string
}

func (User) TableName() string { return "users" }

func (s *ORMIntegrationSuite) SetupSuite() {
	s.Astra.SetT(s.T())
	s.Astra.SetupSuite()
	
	// Create table
	db := s.Astra.DB
	
	s.Require().NotNil(db, "Database should be initialized")
	
	_, err := db.Exec(s.Astra.Ctx, `CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		deleted_at TIMESTAMP WITH TIME ZONE
	)`)
	s.NoError(err)
}

func (s *ORMIntegrationSuite) SetupTest() {
	s.Astra.SetupTest()
}

func (s *ORMIntegrationSuite) TearDownTest() {
	s.Astra.TearDownTest()
}

func (s *ORMIntegrationSuite) TearDownSuite() {
	s.Astra.TearDownSuite()
}

func (s *ORMIntegrationSuite) TestCursorPagination() {
	db := s.Astra.DB
	ctx := context.Background()

	// Seed data
	for i := 1; i <= 20; i++ {
		_, err := database.Query[User](db).Create(&User{
			Email: fmt.Sprintf("user%d@example.com", i),
			Name:  fmt.Sprintf("User %d", i),
		}, ctx)
		s.NoError(err)
	}

	// First page
	result, err := database.Query[User](db).
		OrderBy("id", "asc").
		CursorPaginate(ctx, "id", "", 5)
	s.NoError(err)
	s.Len(result.Data, 5)
	s.True(result.HasMore)
	s.NotEmpty(result.NextCursor)

	// Second page
	result2, err := database.Query[User](db).
		OrderBy("id", "asc").
		CursorPaginate(ctx, "id", result.NextCursor, 5)
	s.NoError(err)
	s.Len(result2.Data, 5)
	s.NotEqual(result.Data[0].ID, result2.Data[0].ID)
}

func (s *ORMIntegrationSuite) TestFirstOrCreate() {
	db := s.Astra.DB
	ctx := context.Background()

	email := "firstorcreate@example.com"
	
	// Should create
	user, created, err := database.Query[User](db).
		Where("email", "=", email).
		FirstOrCreate(&User{Email: email, Name: "Tester"}, ctx)
	s.NoError(err)
	s.True(created)
	s.Equal("Tester", user.Name)

	// Should fetch existing
	user2, created2, err := database.Query[User](db).
		Where("email", "=", email).
		FirstOrCreate(&User{Email: email, Name: "New Name"}, ctx)
	s.NoError(err)
	s.False(created2)
	s.Equal("Tester", user2.Name)
	s.Equal(user.ID, user2.ID)
}

func TestORMIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite.Run(t, new(ORMIntegrationSuite))
}
