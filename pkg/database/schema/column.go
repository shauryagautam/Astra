package schema

type Column struct {
	Name           string
	Type           string
	IsNullable     bool
	IsUnique       bool
	IsPrimary      bool
	IsAuto         bool
	DefaultValue   any
	ReferenceTable string
	ReferenceCol   string
}

func (c *Column) Nullable() *Column {
	c.IsNullable = true
	return c
}

func (c *Column) NotNull() *Column {
	c.IsNullable = false
	return c
}

func (c *Column) Default(val any) *Column {
	c.DefaultValue = val
	return c
}

func (c *Column) Unique() *Column {
	c.IsUnique = true
	return c
}

func (c *Column) Primary() *Column {
	c.IsPrimary = true
	return c
}

func (c *Column) AutoIncrement() *Column {
	c.IsAuto = true
	return c
}

func (c *Column) References(table, column string) *Column {
	c.ReferenceTable = table
	c.ReferenceCol = column
	return c
}
