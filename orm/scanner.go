package orm

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"
)

// CustomScanner allows types to control how they are scanned from the database.
type CustomScanner interface {
	ScanValue(src any) error
}

// scanRows scans Rows into a []T using the model metadata.
// Returns a reflect.Value of kind Slice whose element type is meta.Type.
func (db *DB) scanRows(rows Rows, meta *ModelMeta) (any, error) {
	defer rows.Close()

	sliceType := reflect.SliceOf(meta.Type)
	slice := reflect.MakeSlice(sliceType, 0, 16)

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Build a column→ColumnMeta lookup ordered by the result set.
	colMetas := make([]ColumnMeta, len(columns))
	colValid := make([]bool, len(columns))
	for i, name := range columns {
		if cm, ok := meta.ColumnByCol[name]; ok {
			colMetas[i] = cm
			colValid[i] = true
		}
	}

	for rows.Next() {
		itemPtr := reflect.New(meta.Type)
		item := itemPtr.Elem()

		if err := db.scanRow(rows, columns, colMetas, colValid, item); err != nil {
			return nil, err
		}
		slice = reflect.Append(slice, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return slice.Interface(), nil
}

// scanRow scans one Row into an already-allocated reflect.Value.
// Uses reflect.Value.FieldByIndex — safe, no unsafe pointer arithmetic.
func (db *DB) scanRow(row Row, columns []string, colMetas []ColumnMeta, colValid []bool, item reflect.Value) error {
	targets := make([]any, len(columns))

	for i := range columns {
		if !colValid[i] {
			// Skip unknown columns gracefully.
			targets[i] = new(sql.RawBytes)
			continue
		}
		cm := colMetas[i]
		field := fieldByIndex(item, cm.FieldIndex)
		targets[i] = scanTarget(field)
	}

	return row.Scan(targets...)
}

// scanTarget returns a pointer suitable for sql.Rows.Scan into field f.
// Uses addr-of the field where possible (zero allocation for primitives),
// or a nullablePtr wrapper for pointer fields (handles NULL→nil).
func scanTarget(f reflect.Value) any {
	if !f.CanAddr() {
		return new(any)
	}

	// Let types that implement sql.Scanner handle themselves (e.g. Encrypted[T]).
	if f.CanInterface() {
		iface := f.Addr().Interface()
		if _, ok := iface.(sql.Scanner); ok {
			return iface
		}
	}

	switch f.Kind() {
	case reflect.Ptr:
		// For nullable pointer fields (e.g. *time.Time), we wrap in a
		// nullablePtr so the driver can store nil (NULL) without panicking.
		return &nullablePtr{field: f}
	default:
		// For concrete types, use a convertingScanner that handles driver type
		// mismatches (e.g. int64→uint, int64→int, string→[]byte, etc.).
		return &convertingScanner{field: f}
	}
}

// convertingScanner handles scanning when the driver type may differ from
// the struct field type (e.g. SQLite returns int64 for all integers,
// but the field may be uint, int, int32, etc.).
type convertingScanner struct {
	field reflect.Value // addressable concrete field (non-pointer)
}

func (c *convertingScanner) Scan(src any) error {
	if src == nil {
		c.field.Set(reflect.Zero(c.field.Type()))
		return nil
	}

	// Fast path: direct assignability (most fields will match).
	srcVal := reflect.ValueOf(src)
	if srcVal.Type().AssignableTo(c.field.Type()) {
		c.field.Set(srcVal)
		return nil
	}

	// Slow path: type conversion (int64→uint, etc.).
	if srcVal.Type().ConvertibleTo(c.field.Type()) {
		c.field.Set(srcVal.Convert(c.field.Type()))
		return nil
	}

	// Let the stdlib scanner handle it (time.Time from string etc.).
	iface := c.field.Addr().Interface()
	if scanner, ok := iface.(sql.Scanner); ok {
		return scanner.Scan(src)
	}

	return fmt.Errorf("orm: cannot scan %T (%v) into %s", src, src, c.field.Type())

}

// nullablePtr is a sql.Scanner adapter for pointer fields.
// It allows the driver to scan nil (NULL) into a *T field by allocating
// pointer only when the scanned value is non-nil.
type nullablePtr struct {
	field reflect.Value // must be a reflect.Value of kind Ptr
}

func (n *nullablePtr) Scan(src any) error {
	if src == nil {
		// NULL → nil pointer; field is already nil (zero value of pointer type).
		n.field.Set(reflect.Zero(n.field.Type()))
		return nil
	}
	// Allocate the pointed-to type and scan into it.
	ptr := reflect.New(n.field.Type().Elem())
	iface := ptr.Interface()
	if scanner, ok := iface.(sql.Scanner); ok {
		if err := scanner.Scan(src); err != nil {
			return err
		}
	} else {
		// Use convertible assignment for basic types (int64→int, etc.).
		srcVal := reflect.ValueOf(src)
		elemType := n.field.Type().Elem()
		if !srcVal.Type().AssignableTo(elemType) {
			if !srcVal.Type().ConvertibleTo(elemType) {
				return fmt.Errorf("orm: cannot scan %T into %s", src, elemType)
			}
			ptr.Elem().Set(srcVal.Convert(elemType))
		} else {
			ptr.Elem().Set(srcVal)
		}
	}
	n.field.Set(ptr)
	return nil
}

// scanInto scans rows into dest, which must be a pointer to a slice of structs.
// Used by RawQuery.Scan.
func scanInto(rows Rows, dest any) error {
	defer rows.Close()

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("orm: scanInto requires a *[]T destination, got %T", dest)
	}

	sliceVal := destVal.Elem()
	elemType := sliceVal.Type().Elem()
	isPtr := elemType.Kind() == reflect.Ptr
	if isPtr {
		elemType = elemType.Elem()
	}

	if elemType.Kind() != reflect.Struct {
		return fmt.Errorf("orm: scanInto slice element must be a struct, got %s", elemType.Kind())
	}

	meta := GetMeta(elemType)
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	colMetas := make([]ColumnMeta, len(columns))
	colValid := make([]bool, len(columns))
	for i, name := range columns {
		if cm, ok := meta.ColumnByCol[name]; ok {
			colMetas[i] = cm
			colValid[i] = true
		}
	}

	// dummy DB instance — scanRow only uses meta resolution helpers.
	dbProxy := &DB{}

	for rows.Next() {
		itemPtr := reflect.New(meta.Type)
		item := itemPtr.Elem()

		if err := dbProxy.scanRow(rows, columns, colMetas, colValid, item); err != nil {
			return err
		}

		if isPtr {
			sliceVal.Set(reflect.Append(sliceVal, itemPtr))
		} else {
			sliceVal.Set(reflect.Append(sliceVal, item))
		}
	}

	return rows.Err()
}

// scanOneInto scans the first row into dest (pointer to struct).
func scanOneInto(rows Rows, dest any) error {
	defer rows.Close()

	destVal := reflect.ValueOf(dest)
	if destVal.Kind() != reflect.Ptr || destVal.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("orm: ScanOne requires a *T destination, got %T", dest)
	}

	meta := GetMeta(destVal.Elem().Type())
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	colMetas := make([]ColumnMeta, len(columns))
	colValid := make([]bool, len(columns))
	for i, name := range columns {
		if cm, ok := meta.ColumnByCol[name]; ok {
			colMetas[i] = cm
			colValid[i] = true
		}
	}

	if !rows.Next() {
		return fmt.Errorf("orm: ScanOne: no rows in result set")
	}

	dbProxy := &DB{}
	item := destVal.Elem()
	if err := dbProxy.scanRow(rows, columns, colMetas, colValid, item); err != nil {
		return err
	}
	return rows.Err()
}

// ensure time.Time is always recognised by the type system (import used).
var _ = time.Now
