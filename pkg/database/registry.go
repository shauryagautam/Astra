package database

import (
	"reflect"
	"strings"
	"sync"
)

var registry sync.Map // map[reflect.Type]*ModelMeta

// ModelMeta holds pre-computed metadata for a model.
type ModelMeta struct {
	Type        reflect.Type
	TableName   string
	Columns     []ColumnMeta
	ColumnByCol map[string]ColumnMeta
	PK          ColumnMeta
	HasSoftDel  bool
	Relations   []RelationMeta
}

// ColumnMeta holds metadata for a single column/field.
// FieldIndex is a multi-level index compatible with reflect.Value.FieldByIndex,
// which correctly handles embedded structs without any unsafe pointer arithmetic.
type ColumnMeta struct {
	FieldName  string
	ColumnName string
	FieldIndex []int // replaces uintptr Offset — safe, GC-correct
	IsPK       bool
	IsAuto     bool
	IsSoftDel  bool
	IsGuarded  bool // Mass assignment protection
	IsNullZero bool
	Type       reflect.Type
}

// RelationMeta holds metadata for a model relation.
type RelationMeta struct {
	FieldName  string
	Type       string // "has_one", "has_many", "belongs_to", "many_to_many"
	Related    reflect.Type
	FK         string
	RelatedKey string // FK on the related side for many_to_many
	Pivot      string // pivot table name
	MorphType  string // for polymorphic relations
	MorphID    string // for polymorphic relations
}

// fieldByIndex returns a reflect.Value for a (possibly embedded) field
// identified by a multi-level field index. The value must be addressable
// (i.e. obtained from a pointer).
func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

// RegisterModel manually adds metadata for a type, typically called by generated code.
func RegisterModel[T any](meta ModelMeta) {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		t = reflect.TypeOf(&zero).Elem()
	}
	// Ensure TableName and PK are set if not provided.
	if meta.TableName == "" {
		meta.TableName = getTableName(t)
	}
	if meta.Type == nil {
		meta.Type = t
	}
	if meta.ColumnByCol == nil {
		meta.ColumnByCol = make(map[string]ColumnMeta)
		for _, col := range meta.Columns {
			meta.ColumnByCol[col.ColumnName] = col
			if col.IsPK {
				meta.PK = col
			}
		}
	}
	registry.Store(t, &meta)
}

// GetMeta retrieves or builds metadata for a type.
func GetMeta(t reflect.Type) *ModelMeta {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if val, ok := registry.Load(t); ok {
		return val.(*ModelMeta)
	}
	// Fallback to runtime reflection (deprecated Path).
	meta := buildMeta(t, nil)
	val, _ := registry.LoadOrStore(t, meta)
	return val.(*ModelMeta)
}

// buildMeta recursively constructs ModelMeta for t.
// Deprecated: Use static registration via generate:glue.
func buildMeta(t reflect.Type, parentIndex []int) *ModelMeta {
	meta := &ModelMeta{
		Type:        t,
		TableName:   getTableName(t),
		ColumnByCol: make(map[string]ColumnMeta),
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("orm")
		dbTag := field.Tag.Get("db")

		if tag == "-" || dbTag == "-" {
			continue
		}

		currentIndex := append(append([]int(nil), parentIndex...), i)

		// Handle embedded structs (e.g., embedded Model).
		if field.Anonymous {
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				embedded := buildMeta(ft, currentIndex)
				for _, col := range embedded.Columns {
					meta.Columns = append(meta.Columns, col)
					meta.ColumnByCol[col.ColumnName] = col
					if col.IsPK {
						meta.PK = col
					}
					if col.IsSoftDel {
						meta.HasSoftDel = true
					}
				}
				meta.Relations = append(meta.Relations, embedded.Relations...)
				continue
			}
		}

		if isRelationType(field.Type) {
			meta.Relations = append(meta.Relations, parseRelation(field))
			continue
		}

		col := parseColumn(field, tag, currentIndex)
		meta.Columns = append(meta.Columns, col)
		meta.ColumnByCol[col.ColumnName] = col
		if col.IsPK {
			meta.PK = col
		}
		if col.IsSoftDel {
			meta.HasSoftDel = true
		}
	}

	return meta
}

func parseColumn(field reflect.StructField, tag string, index []int) ColumnMeta {
	col := ColumnMeta{
		FieldName:  field.Name,
		ColumnName: toSnakeCase(field.Name),
		FieldIndex: index,
		Type:       field.Type,
	}

	if tag == "" {
		return col
	}

	for _, part := range strings.Split(tag, ";") {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		key := kv[0]
		var val string
		if len(kv) > 1 {
			val = kv[1]
		}

		switch key {
		case "column":
			if val != "" {
				col.ColumnName = val
			}
		case "primaryKey", "primary_key":
			col.IsPK = true
		case "autoIncrement", "auto_increment":
			col.IsAuto = true
		case "soft_delete":
			col.IsSoftDel = true
		case "guarded", "protected":
			col.IsGuarded = true
		case "not_null", "unique":
			// reserved for future schema builder use
		case "null_zero":
			col.IsNullZero = true
		}
	}

	return col
}

func parseRelation(field reflect.StructField) RelationMeta {
	rel := RelationMeta{
		FieldName: field.Name,
	}

	ft := field.Type
	switch ft.Kind() {
	case reflect.Slice:
		rel.Related = ft.Elem()
	case reflect.Ptr:
		rel.Related = ft.Elem()
	case reflect.Struct:
		rel.Related = ft
	}

	tag := field.Tag.Get("orm")
	if tag != "" {
		for _, part := range strings.Split(tag, ";") {
			kv := strings.SplitN(part, ":", 2)
			key := kv[0]
			var val string
			if len(kv) > 1 {
				val = kv[1]
			}
			switch key {
			case "hasMany":
				rel.Type = "has_many"
			case "hasOne":
				rel.Type = "has_one"
			case "belongsTo":
				rel.Type = "belongs_to"
			case "manyToMany":
				rel.Type = "many_to_many"
			case "morphTo":
				rel.Type = "morph_to"
			case "morphMany":
				rel.Type = "morph_many"
			case "morphType":
				rel.MorphType = val
			case "morphID":
				rel.MorphID = val
			case "foreignKey":
				rel.FK = val
			case "relatedKey":
				rel.RelatedKey = val
			case "pivot":
				rel.Pivot = val
			}
		}
	}

	// Fallback: infer type from generic wrapper type name.
	if rel.Type == "" {
		switch ft.Name() {
		case "HasMany":
			rel.Type = "has_many"
			if ft.NumField() > 1 {
				rel.Related = ft.Field(1).Type.Elem()
			}
		case "HasOne":
			rel.Type = "has_one"
			if ft.NumField() > 1 {
				rel.Related = ft.Field(1).Type
			}
		case "BelongsTo":
			rel.Type = "belongs_to"
			if ft.NumField() > 1 {
				rel.Related = ft.Field(1).Type
			}
		case "ManyToMany":
			rel.Type = "many_to_many"
			if ft.NumField() > 1 {
				rel.Related = ft.Field(1).Type.Elem()
			}
		case "MorphTo":
			rel.Type = "morph_to"
		case "MorphMany":
			rel.Type = "morph_many"
			if ft.NumField() > 1 {
				rel.Related = ft.Field(1).Type.Elem()
			}
		}
	}

	return rel
}

func isRelationType(t reflect.Type) bool {
	n := t.Name()
	return n == "HasMany" || n == "HasOne" || n == "BelongsTo" || n == "ManyToMany" || n == "MorphTo" || n == "MorphMany"
}

func getTableName(t reflect.Type) string {
	val := reflect.New(t).Interface()
	if tn, ok := val.(TableNamer); ok {
		return tn.TableName()
	}
	return toSnakeCase(t.Name()) + "s"
}

// toSnakeCase converts CamelCase to snake_case.
// Handles consecutive uppercase abbreviations:
//
//	ID → id, URL → url, UserID → user_id, CreatedAt → created_at
func toSnakeCase(s string) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return ""
	}
	var result strings.Builder
	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if !isUpper {
			result.WriteRune(r)
			continue
		}
		// Determine whether to insert an underscore before this letter:
		// Yes if: not the first character, AND either
		//   (a) previous character was lowercase, OR
		//   (b) next character is lowercase (end of an abbreviation run like "URL" → "ur_l" would be wrong,
		//       so we check: we're inside an abbreviation if both prev and next are upper).
		if i > 0 {
			prevUpper := runes[i-1] >= 'A' && runes[i-1] <= 'Z'
			nextLower := i+1 < n && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			if !prevUpper || nextLower {
				result.WriteRune('_')
			}
		}
		result.WriteRune(r | 0x20) // to lower
	}
	return result.String()
}
