package orm_test

import (
	"reflect"
	"testing"

	"github.com/astraframework/astra/orm"
	"github.com/stretchr/testify/assert"
)

type ProtectedUser struct {
	orm.Model
	Name    string `orm:"column:name"`
	IsAdmin bool   `orm:"column:is_admin;guarded"`
}

func (ProtectedUser) TableName() string { return "protected_users" }

func TestMassAssignment(t *testing.T) {
	// This test assumes a working DB connection or can be run as a unit test
	// if we just check the generated SQL or the internal meta.

	meta := orm.GetMeta(reflect.TypeOf(ProtectedUser{}))

	var isAdminCol *orm.ColumnMeta
	for _, col := range meta.Columns {
		if col.ColumnName == "is_admin" {
			isAdminCol = &col
			break
		}
	}

	assert.NotNil(t, isAdminCol)
	assert.True(t, isAdminCol.IsGuarded)
}
