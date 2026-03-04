package db

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// QueryBuilder is a fluent SQL query builder using generics.
type QueryBuilder[T any] struct {
	pool       *pgxpool.Pool
	table      string
	conditions []string
	args       []any
	order      string
	limit      int
	offset     int
	relations  []string
}

// NewQueryBuilder creates a new QueryBuilder for type T.
func NewQueryBuilder[T any](pool *pgxpool.Pool, table string) *QueryBuilder[T] {
	return &QueryBuilder[T]{
		pool:       pool,
		table:      table,
		conditions: make([]string, 0),
		args:       make([]any, 0),
		relations:  make([]string, 0),
	}
}

// Where adds a WHERE clause.
func (qb *QueryBuilder[T]) Where(condition string, args ...any) *QueryBuilder[T] {
	qb.conditions = append(qb.conditions, condition)
	qb.args = append(qb.args, args...)
	return qb
}

// OrWhere adds an OR WHERE clause.
func (qb *QueryBuilder[T]) OrWhere(condition string, args ...any) *QueryBuilder[T] {
	if len(qb.conditions) == 0 {
		return qb.Where(condition, args...)
	}
	qb.conditions = append(qb.conditions, "OR "+condition)
	qb.args = append(qb.args, args...)
	return qb
}

// WhereIn adds a WHERE IN clause.
func (qb *QueryBuilder[T]) WhereIn(col string, vals []any) *QueryBuilder[T] {
	if len(vals) == 0 {
		qb.conditions = append(qb.conditions, "1=0") // impossible condition
		return qb
	}
	placeholders := make([]string, len(vals))
	for i := range vals {
		placeholders[i] = fmt.Sprintf("$%d", len(qb.args)+i+1)
	}
	qb.conditions = append(qb.conditions, fmt.Sprintf("%s IN (%s)", col, strings.Join(placeholders, ", ")))
	qb.args = append(qb.args, vals...)
	return qb
}

// With adds relations to eager load.
func (qb *QueryBuilder[T]) With(relations ...string) *QueryBuilder[T] {
	qb.relations = append(qb.relations, relations...)
	return qb
}

// OrderBy adds an ORDER BY clause.
func (qb *QueryBuilder[T]) OrderBy(col string, dir ...string) *QueryBuilder[T] {
	d := "ASC"
	if len(dir) > 0 && strings.ToUpper(dir[0]) == "DESC" {
		d = "DESC"
	}
	qb.order = fmt.Sprintf("%s %s", col, d)
	return qb
}

// Limit adds a LIMIT clause.
func (qb *QueryBuilder[T]) Limit(n int) *QueryBuilder[T] {
	qb.limit = n
	return qb
}

// Offset adds an OFFSET clause.
func (qb *QueryBuilder[T]) Offset(n int) *QueryBuilder[T] {
	qb.offset = n
	return qb
}

// buildSelect constructs the SELECT query.
func (qb *QueryBuilder[T]) buildSelect(count bool) string {
	var sb strings.Builder
	if count {
		sb.WriteString("SELECT COUNT(*) FROM ")
	} else {
		sb.WriteString("SELECT * FROM ")
	}
	sb.WriteString(qb.table)

	if len(qb.conditions) > 0 {
		sb.WriteString(" WHERE ")
		for i, cond := range qb.conditions {
			if i > 0 && !strings.HasPrefix(cond, "OR ") {
				sb.WriteString(" AND ")
			} else if i > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(cond)
		}
	}

	if !count {
		if qb.order != "" {
			sb.WriteString(" ORDER BY " + qb.order)
		}
		if qb.limit > 0 {
			sb.WriteString(fmt.Sprintf(" LIMIT %d", qb.limit))
		}
		if qb.offset > 0 {
			sb.WriteString(fmt.Sprintf(" OFFSET %d", qb.offset))
		}
	}

	return sb.String()
}

// First executes the query and returns the first result.
func (qb *QueryBuilder[T]) First(ctx context.Context) (*T, error) {
	qb.Limit(1)
	query := qb.buildSelect(false)

	var rows pgx.Rows
	var err error
	if tx := GetTx(ctx); tx != nil {
		rows, err = tx.Query(ctx, query, qb.args...)
	} else {
		rows, err = qb.pool.Query(ctx, query, qb.args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, err
	}

	if len(qb.relations) > 0 {
		if err := qb.eagerLoad(ctx, []T{res}); err != nil {
			return nil, err
		}
	}

	return &res, nil
}

// All executes the query and returns all results.
func (qb *QueryBuilder[T]) All(ctx context.Context) ([]T, error) {
	query := qb.buildSelect(false)

	var rows pgx.Rows
	var err error
	if tx := GetTx(ctx); tx != nil {
		rows, err = tx.Query(ctx, query, qb.args...)
	} else {
		rows, err = qb.pool.Query(ctx, query, qb.args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, err
	}

	if len(qb.relations) > 0 && len(results) > 0 {
		if err := qb.eagerLoad(ctx, results); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// Count returns the total number of records matching the query.
func (qb *QueryBuilder[T]) Count(ctx context.Context) (int64, error) {
	query := qb.buildSelect(true)

	var count int64
	var err error
	if tx := GetTx(ctx); tx != nil {
		err = tx.QueryRow(ctx, query, qb.args...).Scan(&count)
	} else {
		err = qb.pool.QueryRow(ctx, query, qb.args...).Scan(&count)
	}

	if err != nil {
		return 0, err
	}
	return count, nil
}

// Exists returns true if at least one record matches the query.
func (qb *QueryBuilder[T]) Exists(ctx context.Context) (bool, error) {
	count, err := qb.Count(ctx)
	return count > 0, err
}

// Paginate returns a paginated result.
func (qb *QueryBuilder[T]) Paginate(ctx context.Context, page, perPage int) (Paginated[T], error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	total, err := qb.Count(ctx)
	if err != nil {
		return Paginated[T]{}, err
	}

	qb.Limit(perPage).Offset((page - 1) * perPage)
	data, err := qb.All(ctx)
	if err != nil {
		return Paginated[T]{}, err
	}

	lastPage := int(total) / perPage
	if int(total)%perPage != 0 {
		lastPage++
	}

	return Paginated[T]{
		Data:     data,
		Total:    int(total),
		Page:     page,
		PerPage:  perPage,
		LastPage: lastPage,
	}, nil
}

// CursorPaginate returns a cursor-based paginated result.
func (qb *QueryBuilder[T]) CursorPaginate(ctx context.Context, cursorCol, cursor string, limit int) (CursorPaginated[T], error) {
	if limit < 1 {
		limit = 15
	}

	clone := &QueryBuilder[T]{
		pool:       qb.pool,
		table:      qb.table,
		conditions: make([]string, len(qb.conditions)),
		args:       make([]any, len(qb.args)),
		relations:  qb.relations,
	}
	copy(clone.conditions, qb.conditions)
	copy(clone.args, qb.args)

	if cursor != "" {
		argIdx := len(clone.args) + 1
		clone.conditions = append(clone.conditions, fmt.Sprintf("%s > $%d", cursorCol, argIdx))
		clone.args = append(clone.args, cursor)
	}

	clone.OrderBy(cursorCol, "ASC")
	clone.Limit(limit + 1)

	data, err := clone.All(ctx)
	if err != nil {
		return CursorPaginated[T]{}, err
	}

	hasMore := len(data) > limit
	if hasMore {
		data = data[:limit]
	}

	nextCursor := ""
	if hasMore && len(data) > 0 {
		nextCursor = fmt.Sprintf("%v", reflect.ValueOf(data[len(data)-1]).FieldByName(strings.Title(cursorCol)).Interface())
	}

	return CursorPaginated[T]{
		Data:       data,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// Create inserts a new record and returns the created model.
func (qb *QueryBuilder[T]) Create(ctx context.Context, data map[string]any) (*T, error) {
	cols := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	placeholders := make([]string, 0, len(data))

	i := 1
	for k, v := range data {
		cols = append(cols, k)
		vals = append(vals, v)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		i++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *", qb.table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	var rows pgx.Rows
	var err error
	if tx := GetTx(ctx); tx != nil {
		rows, err = tx.Query(ctx, query, vals...)
	} else {
		rows, err = qb.pool.Query(ctx, query, vals...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// BulkInsert inserts multiple records in a single query.
func (qb *QueryBuilder[T]) BulkInsert(ctx context.Context, rowsData []map[string]any) ([]T, error) {
	if len(rowsData) == 0 {
		return nil, nil
	}

	// Assume all rows have the same keys as the first row
	cols := make([]string, 0, len(rowsData[0]))
	for k := range rowsData[0] {
		cols = append(cols, k)
	}

	var placeholders []string
	var vals []any
	argIdx := 1
	for _, data := range rowsData {
		var rowPlaceholders []string
		for _, col := range cols {
			rowPlaceholders = append(rowPlaceholders, fmt.Sprintf("$%d", argIdx))
			vals = append(vals, data[col])
			argIdx++
		}
		placeholders = append(placeholders, "("+strings.Join(rowPlaceholders, ", ")+")")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s RETURNING *", qb.table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	var rows pgx.Rows
	var err error
	if tx := GetTx(ctx); tx != nil {
		rows, err = tx.Query(ctx, query, vals...)
	} else {
		rows, err = qb.pool.Query(ctx, query, vals...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[T])
}

// Upsert performs an INSERT ON CONFLICT UPDATE.
func (qb *QueryBuilder[T]) Upsert(ctx context.Context, data map[string]any, conflictCol string) (*T, error) {
	cols := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	placeholders := make([]string, 0, len(data))
	updateParts := make([]string, 0, len(data))

	i := 1
	for k, v := range data {
		cols = append(cols, k)
		vals = append(vals, v)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		if k != conflictCol {
			updateParts = append(updateParts, fmt.Sprintf("%s = EXCLUDED.%s", k, k))
		}
		i++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s RETURNING *",
		qb.table, strings.Join(cols, ", "), strings.Join(placeholders, ", "), conflictCol, strings.Join(updateParts, ", "))

	var rows pgx.Rows
	var err error
	if tx := GetTx(ctx); tx != nil {
		rows, err = tx.Query(ctx, query, vals...)
	} else {
		rows, err = qb.pool.Query(ctx, query, vals...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Update updates records matching the query.
func (qb *QueryBuilder[T]) Update(ctx context.Context, data map[string]any) (*T, error) {
	if len(qb.conditions) == 0 {
		return nil, fmt.Errorf("update requires at least one condition")
	}

	setParts := make([]string, 0, len(data))
	vals := make([]any, 0, len(data)+len(qb.args))

	i := 1
	for k, v := range data {
		setParts = append(setParts, fmt.Sprintf("%s = $%d", k, i))
		vals = append(vals, v)
		i++
	}

	var where strings.Builder
	where.WriteString(" WHERE ")
	for j, cond := range qb.conditions {
		if j > 0 && !strings.HasPrefix(cond, "OR ") {
			where.WriteString(" AND ")
		} else if j > 0 {
			where.WriteString(" ")
		}
		where.WriteString(cond)
	}

	query := fmt.Sprintf("UPDATE %s SET %s%s RETURNING *", qb.table, strings.Join(setParts, ", "), where.String())
	vals = append(vals, qb.args...)

	var rows pgx.Rows
	var err error
	if tx := GetTx(ctx); tx != nil {
		rows, err = tx.Query(ctx, query, vals...)
	} else {
		rows, err = qb.pool.Query(ctx, query, vals...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[T])
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// Delete deletes records matching the query.
func (qb *QueryBuilder[T]) Delete(ctx context.Context) error {
	if len(qb.conditions) == 0 {
		return fmt.Errorf("delete requires at least one condition")
	}

	var where strings.Builder
	where.WriteString(" WHERE ")
	for j, cond := range qb.conditions {
		if j > 0 && !strings.HasPrefix(cond, "OR ") {
			where.WriteString(" AND ")
		} else if j > 0 {
			where.WriteString(" ")
		}
		where.WriteString(cond)
	}

	query := fmt.Sprintf("DELETE FROM %s%s", qb.table, where.String())

	var err error
	if tx := GetTx(ctx); tx != nil {
		_, err = tx.Exec(ctx, query, qb.args...)
	} else {
		_, err = qb.pool.Exec(ctx, query, qb.args...)
	}
	return err
}

// SoftDelete marks records as deleted.
func (qb *QueryBuilder[T]) SoftDelete(ctx context.Context) error {
	_, err := qb.Update(ctx, map[string]any{"deleted_at": "NOW()"})
	return err
}

// eagerLoad populates requested relations in the records using batch queries.
func (qb *QueryBuilder[T]) eagerLoad(ctx context.Context, records []T) error {
	if len(records) == 0 {
		return nil
	}

	for _, relName := range qb.relations {
		if err := qb.loadRelation(ctx, records, relName); err != nil {
			return err
		}
	}
	return nil
}

func (qb *QueryBuilder[T]) loadRelation(ctx context.Context, records []T, relName string) error {
	first := records[0]
	t := reflect.TypeOf(first)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var field reflect.StructField
	var ok bool
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if strings.EqualFold(f.Name, relName) || f.Tag.Get("json") == relName {
			field = f
			ok = true
			break
		}
	}

	if !ok {
		return fmt.Errorf("relation %q not found on struct %T", relName, first)
	}

	tag := field.Tag.Get("astra")
	if tag == "" {
		return fmt.Errorf("relation %q is missing 'astra' tag on %T", relName, first)
	}

	parts := strings.Split(tag, ",")
	if len(parts) < 2 {
		return fmt.Errorf("invalid astra tag on %T.%s", first, field.Name)
	}

	relType := parts[0]
	targetTable := parts[1]
	fk := ""
	if len(parts) > 2 {
		fk = parts[2]
	}

	switch relType {
	case "has_many":
		return qb.loadHasMany(ctx, records, field, targetTable, fk)
	case "belongs_to":
		return qb.loadBelongsTo(ctx, records, field, targetTable, fk)
	case "has_one":
		return qb.loadHasOne(ctx, records, field, targetTable, fk)
	default:
		return fmt.Errorf("unsupported relation type %q", relType)
	}
}

func (qb *QueryBuilder[T]) loadHasMany(ctx context.Context, records []T, field reflect.StructField, targetTable, fk string) error {
	if fk == "" {
		fk = strings.ToLower(reflect.TypeOf(records[0]).Name()) + "_id"
	}

	ids := make([]any, 0, len(records))
	idMap := make(map[any][]int)
	for i, r := range records {
		id := reflect.ValueOf(r).FieldByName("ID").Interface()
		ids = append(ids, id)
		idMap[id] = append(idMap[id], i)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ANY($1)", targetTable, fk)
	rows, err := qb.pool.Query(ctx, query, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	childType := field.Type.Elem()
	resultsMap := make(map[any]reflect.Value)

	for rows.Next() {
		childPtr := reflect.New(childType)
		if err := scanRowToStruct(rows, childPtr.Interface()); err != nil {
			return err
		}
		childVal := childPtr.Elem()

		fkVal := childVal.FieldByNameFunc(func(n string) bool { return strings.EqualFold(n, fk) })
		if !fkVal.IsValid() {
			continue
		}

		parentID := fkVal.Interface()
		if _, ok := resultsMap[parentID]; !ok {
			resultsMap[parentID] = reflect.MakeSlice(field.Type, 0, 0)
		}
		resultsMap[parentID] = reflect.Append(resultsMap[parentID], childVal)
	}

	for i := range records {
		ptr := reflect.ValueOf(&records[i]).Elem()
		id := ptr.FieldByName("ID").Interface()
		if slice, ok := resultsMap[id]; ok {
			ptr.FieldByName(field.Name).Set(slice)
		}
	}
	return nil
}

func (qb *QueryBuilder[T]) loadBelongsTo(ctx context.Context, records []T, field reflect.StructField, targetTable, fk string) error {
	if fk == "" {
		fk = strings.ToLower(field.Name) + "_id"
	}

	childIDs := make([]any, 0, len(records))
	idMap := make(map[any][]int)
	for i, r := range records {
		id := reflect.ValueOf(r).FieldByNameFunc(func(n string) bool { return strings.EqualFold(n, fk) }).Interface()
		if id != nil {
			childIDs = append(childIDs, id)
			idMap[id] = append(idMap[id], i)
		}
	}

	if len(childIDs) == 0 {
		return nil
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE id = ANY($1)", targetTable)
	rows, err := qb.pool.Query(ctx, query, childIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		childPtr := reflect.New(field.Type)
		if err := scanRowToStruct(rows, childPtr.Interface()); err != nil {
			return err
		}
		childVal := childPtr.Elem()
		id := childVal.FieldByName("ID").Interface()

		if indices, ok := idMap[id]; ok {
			for _, idx := range indices {
				reflect.ValueOf(&records[idx]).Elem().FieldByName(field.Name).Set(childVal)
			}
		}
	}
	return nil
}

func (qb *QueryBuilder[T]) loadHasOne(ctx context.Context, records []T, field reflect.StructField, targetTable, fk string) error {
	return qb.loadHasMany(ctx, records, field, targetTable, fk)
}

func scanRowToStruct(rows pgx.Rows, dest any) error {
	v := reflect.ValueOf(dest).Elem()
	t := v.Type()
	cols := rows.FieldDescriptions()
	scans := make([]any, len(cols))
	for i, col := range cols {
		fName := ""
		for j := 0; j < t.NumField(); j++ {
			f := t.Field(j)
			if strings.EqualFold(f.Name, col.Name) || f.Tag.Get("db") == col.Name || f.Tag.Get("json") == col.Name {
				fName = f.Name
				break
			}
		}
		if fName != "" {
			scans[i] = v.FieldByName(fName).Addr().Interface()
		} else {
			var dummy any
			scans[i] = &dummy
		}
	}
	return rows.Scan(scans...)
}
