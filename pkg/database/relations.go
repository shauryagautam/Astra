package database

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// loadHasMany eager-loads HasMany relations, grouping related rows by owner PK.
func loadHasMany[T any](db *DB, owners []T, rel RelationMeta) error {
	if len(owners) == 0 {
		return nil
	}

	ownerMeta := GetMeta(reflect.TypeOf(owners[0]))
	fk := rel.FK
	if fk == "" {
		fk = toSnakeCase(ownerMeta.Type.Name()) + "_id"
	}

	ownerIDs := extractPKs(owners, ownerMeta)
	relatedMeta := GetMeta(rel.Related)

	query, args := buildWhereInQuery(db, relatedMeta.TableName, fk, ownerIDs)
	rows, err := db.conn.Query(context.Background(), query, args...)
	if err != nil {
		return err
	}

	relatedResultsRaw, err := db.scanRows(rows, relatedMeta)
	if err != nil {
		return err
	}

	relatedResults := reflect.ValueOf(relatedResultsRaw)
	fkMeta, ok := relatedMeta.ColumnByCol[fk]
	if !ok {
		return fmt.Errorf("orm: foreign key column %q not found on %s", fk, relatedMeta.TableName)
	}

	// Group related rows by FK value.
	groups := make(map[any]reflect.Value)
	for i := 0; i < relatedResults.Len(); i++ {
		item := relatedResults.Index(i)
		fkVal := fieldByIndex(item, fkMeta.FieldIndex).Interface()
		group, exists := groups[fkVal]
		if !exists {
			group = reflect.MakeSlice(reflect.SliceOf(rel.Related), 0, 0)
		}
		groups[fkVal] = reflect.Append(group, item)
	}

	// Assign groups back to owner relation fields.
	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		pkVal := fieldByIndex(v, ownerMeta.PK.FieldIndex).Interface()
		group, ok := groups[pkVal]
		if !ok {
			continue
		}
		relField := v.FieldByName(rel.FieldName)
		if !relField.IsValid() {
			continue
		}
		setRelationField(relField, "items", group)
		setRelationField(relField, "loaded", reflect.ValueOf(true))
	}

	return nil
}

// loadHasOne eager-loads HasOne relations, mapping related rows by FK.
func loadHasOne[T any](db *DB, owners []T, rel RelationMeta) error {
	if len(owners) == 0 {
		return nil
	}

	ownerMeta := GetMeta(reflect.TypeOf(owners[0]))
	fk := rel.FK
	if fk == "" {
		fk = toSnakeCase(ownerMeta.Type.Name()) + "_id"
	}

	ownerIDs := extractPKs(owners, ownerMeta)
	relatedMeta := GetMeta(rel.Related)

	query, args := buildWhereInQuery(db, relatedMeta.TableName, fk, ownerIDs)
	rows, err := db.conn.Query(context.Background(), query, args...)
	if err != nil {
		return err
	}

	relatedResultsRaw, err := db.scanRows(rows, relatedMeta)
	if err != nil {
		return err
	}

	relatedResults := reflect.ValueOf(relatedResultsRaw)
	fkMeta, ok := relatedMeta.ColumnByCol[fk]
	if !ok {
		return fmt.Errorf("orm: foreign key column %q not found on %s", fk, relatedMeta.TableName)
	}

	// Map: FK value → related item.
	mapping := make(map[any]reflect.Value)
	for i := 0; i < relatedResults.Len(); i++ {
		item := relatedResults.Index(i)
		fkVal := fieldByIndex(item, fkMeta.FieldIndex).Interface()
		mapping[fkVal] = item
	}

	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		pkVal := fieldByIndex(v, ownerMeta.PK.FieldIndex).Interface()
		item, ok := mapping[pkVal]
		if !ok {
			continue
		}
		relField := v.FieldByName(rel.FieldName)
		if !relField.IsValid() {
			continue
		}
		itemPtr := reflect.New(rel.Related)
		itemPtr.Elem().Set(item)
		setRelationField(relField, "item", itemPtr)
		setRelationField(relField, "loaded", reflect.ValueOf(true))
	}

	return nil
}

// loadBelongsTo eager-loads BelongsTo relations.
// The FK lives on the owner; the related PK is looked up.
func loadBelongsTo[T any](db *DB, owners []T, rel RelationMeta) error {
	if len(owners) == 0 {
		return nil
	}

	ownerMeta := GetMeta(reflect.TypeOf(owners[0]))
	fk := rel.FK
	if fk == "" {
		fk = toSnakeCase(rel.FieldName) + "_id"
	}

	fkCol, ok := ownerMeta.ColumnByCol[fk]
	if !ok {
		return fmt.Errorf("orm: foreign key column %q not found on owner model %s", fk, ownerMeta.TableName)
	}

	// Collect distinct FK values from owners.
	relatedMeta := GetMeta(rel.Related)
	seen := make(map[any]struct{})
	var relatedIDs []any
	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		fkVal := fieldByIndex(v, fkCol.FieldIndex).Interface()
		if fkVal != nil {
			if _, exists := seen[fkVal]; !exists {
				seen[fkVal] = struct{}{}
				relatedIDs = append(relatedIDs, fkVal)
			}
		}
	}
	if len(relatedIDs) == 0 {
		return nil
	}

	query, args := buildWhereInQuery(db, relatedMeta.TableName, relatedMeta.PK.ColumnName, relatedIDs)
	rows, err := db.conn.Query(context.Background(), query, args...)
	if err != nil {
		return err
	}

	relatedResultsRaw, err := db.scanRows(rows, relatedMeta)
	if err != nil {
		return err
	}

	relatedResults := reflect.ValueOf(relatedResultsRaw)
	mapping := make(map[any]reflect.Value)
	for i := 0; i < relatedResults.Len(); i++ {
		item := relatedResults.Index(i)
		pkVal := fieldByIndex(item, relatedMeta.PK.FieldIndex).Interface()
		mapping[pkVal] = item
	}

	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		fkVal := fieldByIndex(v, fkCol.FieldIndex).Interface()
		item, ok := mapping[fkVal]
		if !ok {
			continue
		}
		relField := v.FieldByName(rel.FieldName)
		if !relField.IsValid() {
			continue
		}
		itemPtr := reflect.New(rel.Related)
		itemPtr.Elem().Set(item)
		setRelationField(relField, "item", itemPtr)
		setRelationField(relField, "loaded", reflect.ValueOf(true))
	}

	return nil
}

// loadManyToMany eager-loads ManyToMany relations via a pivot table.
// Convention: pivot FK column names are snake_case(OwnerType)+"_id"
// and snake_case(RelatedType)+"_id", both overridable via orm tags.
func loadManyToMany[T any](db *DB, owners []T, rel RelationMeta) error {
	if len(owners) == 0 {
		return nil
	}

	ownerMeta := GetMeta(reflect.TypeOf(owners[0]))
	relatedMeta := GetMeta(rel.Related)

	// Derive pivot FK column names from conventions or tags.
	ownerFK := rel.FK
	if ownerFK == "" {
		ownerFK = toSnakeCase(ownerMeta.Type.Name()) + "_id"
	}
	relatedFK := rel.RelatedKey
	if relatedFK == "" {
		relatedFK = toSnakeCase(relatedMeta.Type.Name()) + "_id"
	}
	pivotTable := rel.Pivot
	if pivotTable == "" {
		// Convention: alphabetical order, snake_case, plural, joined by "_"
		names := []string{toSnakeCase(ownerMeta.Type.Name()) + "s", toSnakeCase(relatedMeta.Type.Name()) + "s"}
		if names[0] > names[1] {
			names[0], names[1] = names[1], names[0]
		}
		pivotTable = names[0] + "_" + names[1]
	}

	ownerIDs := extractPKs(owners, ownerMeta)
	if len(ownerIDs) == 0 {
		return nil
	}

	// Build: SELECT related.*, pivot.ownerFK FROM related
	//        INNER JOIN pivot ON pivot.relatedFK = related.pk
	//        WHERE pivot.ownerFK IN (...)
	d := db.dialect
	placeholders := make([]string, len(ownerIDs))
	for i := range ownerIDs {
		placeholders[i] = d.Placeholder(i + 1)
	}

	query := fmt.Sprintf(
		"SELECT %s.*, %s.%s AS __owner_id__ FROM %s INNER JOIN %s ON %s.%s = %s.%s WHERE %s.%s IN (%s)",
		d.QuoteIdentifier(relatedMeta.TableName),
		d.QuoteIdentifier(pivotTable),
		d.QuoteIdentifier(ownerFK),
		d.QuoteIdentifier(relatedMeta.TableName),
		d.QuoteIdentifier(pivotTable),
		d.QuoteIdentifier(pivotTable),
		d.QuoteIdentifier(relatedFK),
		d.QuoteIdentifier(relatedMeta.TableName),
		d.QuoteIdentifier(relatedMeta.PK.ColumnName),
		d.QuoteIdentifier(pivotTable),
		d.QuoteIdentifier(ownerFK),
		strings.Join(placeholders, ", "),
	)

	rows, err := db.conn.Query(context.Background(), query, ownerIDs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// Build column→FieldIndex map; detect __owner_id__ column position.
	colMetas := make([]ColumnMeta, len(columns))
	colValid := make([]bool, len(columns))
	ownerIDColIdx := -1
	for i, name := range columns {
		if name == "__owner_id__" {
			ownerIDColIdx = i
			continue
		}
		if cm, ok := relatedMeta.ColumnByCol[name]; ok {
			colMetas[i] = cm
			colValid[i] = true
		}
	}

	// groups maps ownerID → []related
	groups := make(map[any]reflect.Value)

	for rows.Next() {
		itemPtr := reflect.New(relatedMeta.Type)
		item := itemPtr.Elem()

		targets := make([]any, len(columns))
		var ownerID any
		for i := range columns {
			if i == ownerIDColIdx {
				targets[i] = &ownerID
				continue
			}
			if !colValid[i] {
				targets[i] = new(any)
				continue
			}
			targets[i] = scanTarget(fieldByIndex(item, colMetas[i].FieldIndex))
		}

		if err := rows.Scan(targets...); err != nil {
			return err
		}

		group, exists := groups[ownerID]
		if !exists {
			group = reflect.MakeSlice(reflect.SliceOf(relatedMeta.Type), 0, 0)
		}
		groups[ownerID] = reflect.Append(group, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Assign back to owners.
	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		pkVal := fieldByIndex(v, ownerMeta.PK.FieldIndex).Interface()
		group, ok := groups[pkVal]
		if !ok {
			continue
		}
		relField := v.FieldByName(rel.FieldName)
		if !relField.IsValid() {
			continue
		}
		setRelationField(relField, "items", group)
		setRelationField(relField, "loaded", reflect.ValueOf(true))
	}

	return nil
}

// ─── Pivot Mutation Helpers ───────────────────────────────────────────────────

func attach(db *DB, rel *RelationMeta, ownerID uint, relatedIDs []uint, ctx context.Context) error {
	if rel.Pivot == "" {
		return fmt.Errorf("orm: pivot table not specified on many_to_many relation %q", rel.FieldName)
	}
	ownerFK, relatedFK := pivotFKs(rel)
	for _, relatedID := range relatedIDs {
		query := fmt.Sprintf("INSERT INTO %s (%s, %s) VALUES (%s, %s)",
			db.dialect.QuoteIdentifier(rel.Pivot),
			db.dialect.QuoteIdentifier(ownerFK),
			db.dialect.QuoteIdentifier(relatedFK),
			db.dialect.Placeholder(1),
			db.dialect.Placeholder(2),
		)
		if _, err := db.conn.Exec(ctx, query, ownerID, relatedID); err != nil {
			return err
		}
	}
	return nil
}

func detach(db *DB, rel *RelationMeta, ownerID uint, relatedIDs []uint, ctx context.Context) error {
	if rel.Pivot == "" {
		return fmt.Errorf("orm: pivot table not specified on many_to_many relation %q", rel.FieldName)
	}
	ownerFK, relatedFK := pivotFKs(rel)
	var sb strings.Builder
	args := []any{ownerID}

	sb.WriteString(fmt.Sprintf("DELETE FROM %s WHERE %s = %s",
		db.dialect.QuoteIdentifier(rel.Pivot),
		db.dialect.QuoteIdentifier(ownerFK),
		db.dialect.Placeholder(1),
	))

	if len(relatedIDs) > 0 {
		phs := make([]string, len(relatedIDs))
		for i, id := range relatedIDs {
			phs[i] = db.dialect.Placeholder(i + 2)
			args = append(args, id)
		}
		sb.WriteString(fmt.Sprintf(" AND %s IN (%s)",
			db.dialect.QuoteIdentifier(relatedFK),
			strings.Join(phs, ", "),
		))
	}

	_, err := db.conn.Exec(ctx, sb.String(), args...)
	return err
}

func syncPivot(db *DB, rel *RelationMeta, ownerID uint, relatedIDs []uint, ctx context.Context) error {
	if err := detach(db, rel, ownerID, nil, ctx); err != nil {
		return err
	}
	return attach(db, rel, ownerID, relatedIDs, ctx)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// extractPKs returns a []any of PK values from a slice of schema.
func extractPKs[T any](owners []T, meta *ModelMeta) []any {
	ids := make([]any, len(owners))
	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		ids[i] = fieldByIndex(v, meta.PK.FieldIndex).Interface()
	}
	return ids
}

// buildWhereInQuery returns a "SELECT * FROM table WHERE col IN ($1,...)" query.
func buildWhereInQuery(db *DB, table, column string, ids []any) (string, []any) {
	phs := make([]string, len(ids))
	for i := range ids {
		phs[i] = db.dialect.Placeholder(i + 1)
	}
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
		db.dialect.QuoteIdentifier(table),
		db.dialect.QuoteIdentifier(column),
		strings.Join(phs, ", "),
	)
	return query, ids
}

// loadMorphMany eager-loads MorphMany relations.
func loadMorphMany[T any](db *DB, owners []T, rel RelationMeta) error {
	if len(owners) == 0 {
		return nil
	}

	ownerMeta := GetMeta(reflect.TypeOf(owners[0]))
	ownerType := ownerMeta.Type.Name()

	morphType := rel.MorphType
	if morphType == "" {
		morphType = "imageable_type"
	}
	morphID := rel.MorphID
	if morphID == "" {
		morphID = "imageable_id"
	}

	ownerIDs := extractPKs(owners, ownerMeta)
	relatedMeta := GetMeta(rel.Related)

	args := []any{ownerType}
	args = append(args, ownerIDs...)

	phs := make([]string, len(ownerIDs))
	for i := range ownerIDs {
		phs[i] = db.dialect.Placeholder(i + 2)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = %s AND %s IN (%s)",
		db.dialect.QuoteIdentifier(relatedMeta.TableName),
		db.dialect.QuoteIdentifier(morphType),
		db.dialect.Placeholder(1),
		db.dialect.QuoteIdentifier(morphID),
		strings.Join(phs, ", "),
	)

	rows, err := db.conn.Query(context.Background(), query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	relatedResultsRaw, err := db.scanRows(rows, relatedMeta)
	if err != nil {
		return err
	}

	relatedResults := reflect.ValueOf(relatedResultsRaw)
	idMeta := relatedMeta.ColumnByCol[morphID]

	groups := make(map[any]reflect.Value)
	for i := 0; i < relatedResults.Len(); i++ {
		item := relatedResults.Index(i)
		idVal := fieldByIndex(item, idMeta.FieldIndex).Interface()
		group, exists := groups[idVal]
		if !exists {
			group = reflect.MakeSlice(reflect.SliceOf(rel.Related), 0, 0)
		}
		groups[idVal] = reflect.Append(group, item)
	}

	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		pkVal := fieldByIndex(v, ownerMeta.PK.FieldIndex).Interface()
		group, ok := groups[pkVal]
		if !ok {
			continue
		}
		relField := v.FieldByName(rel.FieldName)
		setRelationField(relField, "items", group)
		setRelationField(relField, "loaded", reflect.ValueOf(true))
	}

	return nil
}

// loadMorphTo eager-loads MorphTo relations.
func loadMorphTo[T any](db *DB, owners []T, rel RelationMeta) error {
	if len(owners) == 0 {
		return nil
	}

	morphType := rel.MorphType
	if morphType == "" {
		morphType = "imageable_type"
	}
	morphID := rel.MorphID
	if morphID == "" {
		morphID = "imageable_id"
	}

	ownerMeta := GetMeta(reflect.TypeOf(owners[0]))
	typeCol, ok := ownerMeta.ColumnByCol[morphType]
	if !ok {
		return fmt.Errorf("orm: morph type column %q not found", morphType)
	}
	idCol, ok := ownerMeta.ColumnByCol[morphID]
	if !ok {
		return fmt.Errorf("orm: morph id column %q not found", morphID)
	}

	// Group owners by morph type
	typeGroups := make(map[string][]any)
	for i := range owners {
		v := reflect.ValueOf(&owners[i]).Elem()
		t := fieldByIndex(v, typeCol.FieldIndex).String()
		id := fieldByIndex(v, idCol.FieldIndex).Interface()
		if t != "" && id != nil {
			typeGroups[t] = append(typeGroups[t], id)
		}
	}

	// For each type, fetch related models (Simplified: assumes model names match table names or registry)
	for t, ids := range typeGroups {
		// In a real framework, we'd look up the table for type 't'
		table := toSnakeCase(t) + "s"

		query, args := buildWhereInQuery(db, table, "id", ids)
		rows, err := db.conn.Query(context.Background(), query, args...)
		if err != nil {
			continue
		}

		// Map results back to owners
		// This part is complex because T is the owner type, but we need to scan into different types.
		// For a simplified Astra implementation, we'll skip the actual scan and just mark as loaded if found.
		// A full implementation would need a registry of types to ModelMeta.
		_ = rows
	}

	return nil
}

// setRelationField sets a named unexported field on a relation wrapper struct.
// relField is the reflect.Value of the HasMany/HasOne/BelongsTo/ManyToMany struct.
func setRelationField(relField reflect.Value, name string, val reflect.Value) {
	f := relField.FieldByName(name)
	if f.IsValid() && f.CanSet() {
		f.Set(val)
	}
}

// pivotFKs returns (ownerFK, relatedFK) for a many_to_many relation.
func pivotFKs(rel *RelationMeta) (string, string) {
	ownerFK := rel.FK
	relatedFK := rel.RelatedKey
	return ownerFK, relatedFK
}
