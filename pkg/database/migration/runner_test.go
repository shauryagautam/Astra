package migration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMigration(t *testing.T) {
	content := `
-- +migrate Up
CREATE TABLE users (id int);
-- +migrate Down
DROP TABLE users;
`
	up, down := parseMigration(content)
	assert.Equal(t, "CREATE TABLE users (id int);", up)
	assert.Equal(t, "DROP TABLE users;", down)

	contentNoDown := `
-- +migrate Up
CREATE TABLE users (id int);
`
	up2, down2 := parseMigration(contentNoDown)
	assert.Equal(t, "CREATE TABLE users (id int);", up2)
	assert.Equal(t, "", down2)
}

func TestComputeChecksum(t *testing.T) {
	c1 := computeChecksum("hello")
	c2 := computeChecksum("hello")
	c3 := computeChecksum("world")

	assert.Equal(t, c1, c2)
	assert.NotEqual(t, c1, c3)
	assert.Len(t, c1, 64) // SHA-256 hex
}

func TestQuoteIdents(t *testing.T) {
	names := []string{"users", "post \"tags\""}
	quoted := quoteIdents(names)
	assert.Equal(t, []string{`"users"`, `"post ""tags"""`}, quoted)
}
