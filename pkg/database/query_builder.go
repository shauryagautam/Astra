package database

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"reflect"
	"strings"
	"time"
)

// QueryBuilder is a generic fluent query builder.
type QueryBuilder[T any] struct {
	db          *DB
	meta        *ModelMeta
	ctx         context.Context
	wheres      []whereClause
	orders      []orderClause
	limit       int
	offset      int
	with         []string
	withTrashed  bool
	baseURL      string
	lock         string
	globalScopes []func(*QueryBuilder[T]) *QueryBuilder[T]
}

type whereClause struct {
	Column   string
	Operator string
	Value    any
	Or       bool
	Raw      string
	Args     []any
}

type orderClause struct {
	Column    string
	Direction string
}

// NewQueryBuilder creates a new instance for a model type.
func NewQueryBuilder[T any](db *DB) *QueryBuilder[T] {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil {
		// T is an interface — get meta from pointer
		t = reflect.TypeOf(&zero).Elem()
	}
	meta := GetMeta(t)

	return &QueryBuilder[T]{
		db:   db,
		meta: meta,
		ctx:  context.Background(),
	}
}

// GlobalScope adds a scope that is applied to all terminal operations.
func (q *QueryBuilder[T]) GlobalScope(fn func(*QueryBuilder[T]) *QueryBuilder[T]) *QueryBuilder[T] {
	q.globalScopes = append(q.globalScopes, fn)
	return q
}

// ApplyScopes runs all registered scopes before SQL generation.
func (q *QueryBuilder[T]) ApplyScopes() *QueryBuilder[T] {
	// Multi-tenancy: Automatically append tenant_id filters if the context contains a tenant_id
	if tenantID, ok := TenantIDFromContext(q.ctx); ok {
		// Only apply if the model actually has a tenant_id column
		for _, col := range q.meta.Columns {
			if col.ColumnName == "tenant_id" {
				q = q.Where("tenant_id", "=", tenantID)
				break
			}
		}
	}

	for _, scope := range q.globalScopes {
		q = scope(q)
	}
	return q
}

// ─── Clause Methods ────────────────────────────────────────────────────────────

func (q *QueryBuilder[T]) Where(column, operator string, value any) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{Column: column, Operator: operator, Value: value})
	return q
}

func (q *QueryBuilder[T]) WhereRaw(raw string, args ...any) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{Raw: raw, Args: args})
	return q
}

func (q *QueryBuilder[T]) OrWhere(column, operator string, value any) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{Column: column, Operator: operator, Value: value, Or: true})
	return q
}

func (q *QueryBuilder[T]) WhereIn(column string, values []any) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{Column: column, Operator: "IN", Value: values})
	return q
}

func (q *QueryBuilder[T]) WhereNull(column string) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{Column: column, Operator: "IS NULL"})
	return q
}

func (q *QueryBuilder[T]) WhereNotNull(column string) *QueryBuilder[T] {
	q.wheres = append(q.wheres, whereClause{Column: column, Operator: "IS NOT NULL"})
	return q
}

func (q *QueryBuilder[T]) OrderBy(column, direction string) *QueryBuilder[T] {
	q.orders = append(q.orders, orderClause{Column: column, Direction: direction})
	return q
}

func (q *QueryBuilder[T]) Limit(n int) *QueryBuilder[T] {
	q.limit = n
	return q
}

func (q *QueryBuilder[T]) Offset(n int) *QueryBuilder[T] {
	q.offset = n
	return q
}

func (q *QueryBuilder[T]) With(relations ...string) *QueryBuilder[T] {
	q.with = append(q.with, relations...)
	return q
}

func (q *QueryBuilder[T]) WithTrashed() *QueryBuilder[T] {
	q.withTrashed = true
	return q
}

func (q *QueryBuilder[T]) Scope(fn func(*QueryBuilder[T]) *QueryBuilder[T]) *QueryBuilder[T] {
	return fn(q)
}

func (q *QueryBuilder[T]) Table(name string) *QueryBuilder[T] {
	q.meta.TableName = name
	return q
}

func (q *QueryBuilder[T]) LockForUpdate() *QueryBuilder[T] {
	q.lock = "FOR UPDATE"
	return q
}

// WithBaseURL sets the base URL for generating pagination links.
func (q *QueryBuilder[T]) WithBaseURL(url string) *QueryBuilder[T] {
	q.baseURL = strings.TrimSuffix(url, "/")
	return q
}

// ─── Terminator Methods ────────────────────────────────────────────────────────

func (q *QueryBuilder[T]) Get(ctx ...context.Context) ([]T, error) {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q = q.ApplyScopes()

	sqlStr, args := q.ToSQL()
	rows, err := q.db.conn.Query(q.ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []T
	for item, err := range ScanIter[T](q.db, rows, q.meta) {
		if err != nil {
			return nil, err
		}
		results = append(results, *item)
	}

	// Eager loading
	if len(q.with) > 0 && len(results) > 0 {
		for _, relName := range q.with {
			if err := q.loadRelation(results, relName); err != nil {
				return nil, err
			}
		}
	}

	// Hook: AfterFind
	for i := range results {
		_ = callAfterFind(q.ctx, q.db, &results[i])
	}

	return results, nil
}

// All returns an iterator over the query results.
// Go 1.23+ iter.Seq2 style.
func (q *QueryBuilder[T]) All(ctx ...context.Context) iter.Seq2[*T, error] {
	return func(yield func(*T, error) bool) {
		if len(ctx) > 0 {
			q.ctx = ctx[0]
		}
		q = q.ApplyScopes()

		sqlStr, args := q.ToSQL()
		rows, err := q.db.conn.Query(q.ctx, sqlStr, args...)
		if err != nil {
			yield(nil, err)
			return
		}
		defer rows.Close()

		for model, err := range ScanIter[T](q.db, rows, q.meta) {
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}

			// Hook: AfterFind
			_ = callAfterFind(q.ctx, q.db, model)

			if !yield(model, nil) {
				return
			}
		}
	}
}

// Each iterates over each result using a callback.
func (q *QueryBuilder[T]) Each(fn func(*T) error, ctx ...context.Context) error {
	for model, err := range q.All(ctx...) {
		if err != nil {
			return err
		}
		if err := fn(model); err != nil {
			return err
		}
	}
	return nil
}

// Chunk processes results in batches of the given size.
func (q *QueryBuilder[T]) Chunk(size int, fn func([]T) error, ctx ...context.Context) error {
	if size <= 0 {
		return fmt.Errorf("chunk size must be greater than zero")
	}

	var batch []T
	for model, err := range q.All(ctx...) {
		if err != nil {
			return err
		}
		batch = append(batch, *model)
		if len(batch) >= size {
			if err := fn(batch); err != nil {
				return err
			}
			batch = nil
		}
	}

	if len(batch) > 0 {
		return fn(batch)
	}
	return nil
}

func (q *QueryBuilder[T]) AllSlice(ctx ...context.Context) ([]T, error) {
	return q.Get(ctx...)
}

func (q *QueryBuilder[T]) First(ctx ...context.Context) (*T, error) {
	q.limit = 1
	results, err := q.Get(ctx...)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, sql.ErrNoRows
	}
	return &results[0], nil
}

func (q *QueryBuilder[T]) Last(ctx ...context.Context) (*T, error) {
	q.limit = 1
	q.OrderBy(q.meta.PK.ColumnName, "DESC")
	return q.First(ctx...)
}

func (q *QueryBuilder[T]) FindByID(id any, ctx ...context.Context) (*T, error) {
	return q.Where(q.meta.PK.ColumnName, "=", id).First(ctx...)
}

func (q *QueryBuilder[T]) FindBy(column string, value any, ctx ...context.Context) (*T, error) {
	return q.Where(column, "=", value).First(ctx...)
}

func (q *QueryBuilder[T]) FirstOrCreate(attributes *T, ctx ...context.Context) (*T, bool, error) {
	found, err := q.First(ctx...)
	if err == nil {
		return found, false, nil
	}
	if err != sql.ErrNoRows {
		return nil, false, err
	}
	res, err := q.Create(attributes, ctx...)
	return res, true, err
}

func (q *QueryBuilder[T]) Count(ctx ...context.Context) (int64, error) {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q = q.ApplyScopes()

	oldLimit, oldOffset := q.limit, q.offset
	q.limit, q.offset = 0, 0
	sqlStr, args := q.toCountSQL()
	q.limit, q.offset = oldLimit, oldOffset

	var count int64
	err := q.db.conn.QueryRow(q.ctx, sqlStr, args...).Scan(&count)
	return count, err
}

func (q *QueryBuilder[T]) Exists(ctx ...context.Context) (bool, error) {
	count, err := q.Count(ctx...)
	return count > 0, err
}

func (q *QueryBuilder[T]) Pluck(column string, ctx ...context.Context) ([]any, error) {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q = q.ApplyScopes()

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(column))
	sb.WriteString(" FROM ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(q.meta.TableName))

	whereStr, args := q.buildWheres(0)
	if whereStr != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereStr)
	}
	if q.limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
	}

	rows, err := q.db.conn.Query(q.ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []any
	for rows.Next() {
		var val any
		if err := rows.Scan(&val); err != nil {
			return nil, err
		}
		result = append(result, val)
	}
	return result, rows.Err()
}

func (q *QueryBuilder[T]) Paginate(page, perPage int, ctx ...context.Context) (*PaginationResult[T], error) {
	total, err := q.Count(ctx...)
	if err != nil {
		return nil, err
	}

	q.Limit(perPage).Offset((page - 1) * perPage)
	data, err := q.Get(ctx...)
	if err != nil {
		return nil, err
	}

	lastPage := int((total + int64(perPage) - 1) / int64(perPage))
	from := (page-1)*perPage + 1
	to := from + len(data) - 1
	if total == 0 {
		from, to = 0, 0
	}

	res := &PaginationResult[T]{
		Data:        data,
		Total:       total,
		PerPage:     perPage,
		CurrentPage: page,
		LastPage:    lastPage,
		From:        from,
		To:          to,
	}

	if q.baseURL != "" {
		res.Links = make(map[string]string)
		if page > 1 {
			res.Links["prev"] = fmt.Sprintf("%s?page=%d&per_page=%d", q.baseURL, page-1, perPage)
		}
		if page < lastPage {
			res.Links["next"] = fmt.Sprintf("%s?page=%d&per_page=%d", q.baseURL, page+1, perPage)
		}
		res.Links["first"] = fmt.Sprintf("%s?page=1&per_page=%d", q.baseURL, perPage)
		res.Links["last"] = fmt.Sprintf("%s?page=%d&per_page=%d", q.baseURL, lastPage, perPage)
	}

	return res, nil
}

// CursorPaginate performs cursor-based pagination.
func (q *QueryBuilder[T]) CursorPaginate(ctx context.Context, column, cursor string, perPage int) (*CursorPaginated[T], error) {
	q.ctx = ctx
	if cursor != "" {
		decoded, err := base64.StdEncoding.DecodeString(cursor)
		if err == nil {
			var cursorMap map[string]any
			if err := json.Unmarshal(decoded, &cursorMap); err == nil {
				if val, ok := cursorMap[column]; ok {
					q.Where(column, ">", val)
				}
			}
		}
	}

	q.Limit(perPage + 1)
	if len(q.orders) == 0 {
		q.OrderBy(column, "ASC")
	}

	data, err := q.Get(ctx)
	if err != nil {
		return nil, err
	}

	hasMore := len(data) > perPage
	if hasMore {
		data = data[:perPage]
	}

	nextCursor := ""
	if hasMore && len(data) > 0 {
		lastItem := data[len(data)-1]
		v := reflect.ValueOf(lastItem)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		// Try to find the field by column name or tag
		// For simplicity, we'll look for a field that matches the column
		var fieldVal any
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			tag := f.Tag.Get("orm")
			if strings.Contains(tag, "column:"+column) || strings.EqualFold(f.Name, column) {
				fieldVal = v.Field(i).Interface()
				break
			}
		}
		if fieldVal == nil {
			// Fallback to primary key if column not found
			fieldVal = fieldByIndex(v, q.meta.PK.FieldIndex).Interface()
		}
		
		cursorData, _ := json.Marshal(map[string]any{column: fieldVal})
		nextCursor = base64.StdEncoding.EncodeToString(cursorData)
	}

	return &CursorPaginated[T]{
		Data:       data,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// ─── Mutation Methods ──────────────────────────────────────────────────────────

func (q *QueryBuilder[T]) Create(model *T, ctx ...context.Context) (*T, error) {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}

	if err := callBeforeCreate(q.ctx, q.db, model); err != nil {
		return nil, err
	}

	v := reflect.ValueOf(model).Elem()
	now := time.Now()
	setTimestamp(v, "CreatedAt", now)
	setTimestamp(v, "UpdatedAt", now)

	var columns []string
	var values []any
	for _, col := range q.meta.Columns {
		if col.IsAuto || col.IsSoftDel || col.IsGuarded {
			continue
		}
		columns = append(columns, col.ColumnName)
		values = append(values, fieldByIndex(v, col.FieldIndex).Interface())
	}

	sqlStr, args := q.toInsertSQL(columns, values)

	if q.db.dialect.SupportsReturning() {
		sqlStr += " RETURNING " + q.db.dialect.QuoteIdentifier(q.meta.PK.ColumnName)
		var id uint
		if err := q.db.conn.QueryRow(q.ctx, sqlStr, args...).Scan(&id); err != nil {
			return nil, err
		}
		setFieldValue(v, q.meta.PK, id)
	} else {
		res, err := q.db.conn.Exec(q.ctx, sqlStr, args...)
		if err != nil {
			return nil, err
		}
		if id, err := res.LastInsertId(); err == nil {
			setFieldValue(v, q.meta.PK, uint(id))
		}
	}

	_ = callAfterCreate(q.ctx, q.db, model)
	return model, nil
}

func (q *QueryBuilder[T]) Update(data map[string]any, ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q = q.ApplyScopes()
	sqlStr, args := q.toUpdateSQL(data)
	_, err := q.db.conn.Exec(q.ctx, sqlStr, args...)
	return err
}

func (q *QueryBuilder[T]) Save(model *T, ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}

	if err := callBeforeUpdate(q.ctx, q.db, model); err != nil {
		return err
	}

	v := reflect.ValueOf(model).Elem()
	setTimestamp(v, "UpdatedAt", time.Now())

	pkVal := fieldByIndex(v, q.meta.PK.FieldIndex).Interface()

	data := make(map[string]any, len(q.meta.Columns))
	for _, col := range q.meta.Columns {
		if col.IsPK || col.IsAuto || col.IsGuarded {
			continue
		}
		data[col.ColumnName] = fieldByIndex(v, col.FieldIndex).Interface()
	}

	q.Where(q.meta.PK.ColumnName, "=", pkVal)
	if err := q.Update(data, q.ctx); err != nil {
		return err
	}

	_ = callAfterUpdate(q.ctx, q.db, model)
	return nil
}

func (q *QueryBuilder[T]) Delete(ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q = q.ApplyScopes()
	if q.meta.HasSoftDel {
		return q.Update(map[string]any{"deleted_at": time.Now()}, q.ctx)
	}
	sqlStr, args := q.toDeleteSQL()
	_, err := q.db.conn.Exec(q.ctx, sqlStr, args...)
	return err
}

func (q *QueryBuilder[T]) ForceDelete(ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q = q.ApplyScopes()
	sqlStr, args := q.toDeleteSQL()
	_, err := q.db.conn.Exec(q.ctx, sqlStr, args...)
	return err
}

func (q *QueryBuilder[T]) Restore(ctx ...context.Context) error {
	if !q.meta.HasSoftDel {
		return fmt.Errorf("orm: model %s does not support soft delete", q.meta.TableName)
	}
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	q.withTrashed = true
	return q.Update(map[string]any{"deleted_at": nil}, q.ctx)
}

// ─── Pivot Operations ─────────────────────────────────────────────────────────

func (q *QueryBuilder[T]) Attach(relation string, ownerID uint, relatedIDs []uint, ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	rel := q.getRelation(relation)
	if rel == nil || rel.Type != "many_to_many" {
		return fmt.Errorf("orm: relation %s is not many_to_many", relation)
	}
	return attach(q.db, rel, ownerID, relatedIDs, q.ctx)
}

func (q *QueryBuilder[T]) Detach(relation string, ownerID uint, relatedIDs []uint, ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	rel := q.getRelation(relation)
	if rel == nil || rel.Type != "many_to_many" {
		return fmt.Errorf("orm: relation %s is not many_to_many", relation)
	}
	return detach(q.db, rel, ownerID, relatedIDs, q.ctx)
}

func (q *QueryBuilder[T]) Sync(relation string, ownerID uint, relatedIDs []uint, ctx ...context.Context) error {
	if len(ctx) > 0 {
		q.ctx = ctx[0]
	}
	rel := q.getRelation(relation)
	if rel == nil || rel.Type != "many_to_many" {
		return fmt.Errorf("orm: relation %s is not many_to_many", relation)
	}
	return syncPivot(q.db, rel, ownerID, relatedIDs, q.ctx)
}

func (q *QueryBuilder[T]) getRelation(name string) *RelationMeta {
	for i := range q.meta.Relations {
		if q.meta.Relations[i].FieldName == name {
			return &q.meta.Relations[i]
		}
	}
	return nil
}

// ─── SQL Generation ───────────────────────────────────────────────────────────

// ToSQL returns the SELECT query string and bound arguments.
func (q *QueryBuilder[T]) ToSQL() (string, []any) {
	return q.buildSelectSQL()
}

func (q *QueryBuilder[T]) buildSelectSQL() (string, []any) {
	var sb strings.Builder
	sb.WriteString("SELECT * FROM ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(q.meta.TableName))

	whereStr, args := q.buildWheres(0)
	if whereStr != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereStr)
	}

	if len(q.orders) > 0 {
		sb.WriteString(" ORDER BY ")
		for i, o := range q.orders {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(q.db.dialect.QuoteIdentifier(o.Column))
			sb.WriteString(" ")
			sb.WriteString(o.Direction)
		}
	}

	if q.limit > 0 {
		if q.offset > 0 {
			sb.WriteString(q.db.dialect.LimitOffsetSQL(q.limit, q.offset))
		} else {
			sb.WriteString(fmt.Sprintf(" LIMIT %d", q.limit))
		}
	}

	if q.lock != "" {
		sb.WriteString(" ")
		sb.WriteString(q.lock)
	}

	return sb.String(), args
}

// buildWheres builds a WHERE clause string with arguments, starting placeholders
// at position (offset + 1). This eliminates the need for buildWheresCustom.
func (q *QueryBuilder[T]) buildWheres(offset int) (string, []any) {
	var sb strings.Builder
	var args []any
	hasClauses := false

	// Automatic soft-delete filter
	if q.meta.HasSoftDel && !q.withTrashed {
		sb.WriteString(q.db.dialect.QuoteIdentifier("deleted_at"))
		sb.WriteString(" IS NULL")
		hasClauses = true
	}

	for _, w := range q.wheres {
		if hasClauses {
			if w.Or {
				sb.WriteString(" OR ")
			} else {
				sb.WriteString(" AND ")
			}
		}

		switch {
		case w.Raw != "":
			sb.WriteString(w.Raw)
			args = append(args, w.Args...)

		case w.Operator == "IN":
			vals := w.Value.([]any)
			sb.WriteString(q.db.dialect.QuoteIdentifier(w.Column))
			sb.WriteString(" IN (")
			for i, v := range vals {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(q.db.dialect.Placeholder(offset + len(args) + 1))
				args = append(args, v)
			}
			sb.WriteString(")")

		case strings.Contains(w.Operator, "NULL"):
			sb.WriteString(q.db.dialect.QuoteIdentifier(w.Column))
			sb.WriteString(" ")
			sb.WriteString(w.Operator)

		default:
			sb.WriteString(q.db.dialect.QuoteIdentifier(w.Column))
			sb.WriteString(" ")
			sb.WriteString(w.Operator)
			sb.WriteString(" ")
			sb.WriteString(q.db.dialect.Placeholder(offset + len(args) + 1))
			args = append(args, w.Value)
		}

		hasClauses = true
	}

	return sb.String(), args
}

func (q *QueryBuilder[T]) toCountSQL() (string, []any) {
	var sb strings.Builder
	sb.WriteString("SELECT COUNT(*) FROM ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(q.meta.TableName))

	whereStr, args := q.buildWheres(0)
	if whereStr != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereStr)
	}
	return sb.String(), args
}

func (q *QueryBuilder[T]) toInsertSQL(columns []string, values []any) (string, []any) {
	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(q.meta.TableName))
	sb.WriteString(" (")
	for i, col := range columns {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(q.db.dialect.QuoteIdentifier(col))
	}
	sb.WriteString(") VALUES (")
	for i := range values {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(q.db.dialect.Placeholder(i + 1))
	}
	sb.WriteString(")")
	return sb.String(), values
}

// toUpdateSQL builds a single-pass UPDATE statement.
// SET placeholders are $1..$n; WHERE placeholders continue from $n+1.
func (q *QueryBuilder[T]) toUpdateSQL(data map[string]any) (string, []any) {
	var sb strings.Builder
	var args []any

	sb.WriteString("UPDATE ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(q.meta.TableName))
	sb.WriteString(" SET ")

	i := 0
	for col, val := range data {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(q.db.dialect.QuoteIdentifier(col))
		sb.WriteString(" = ")
		sb.WriteString(q.db.dialect.Placeholder(i + 1))
		args = append(args, val)
		i++
	}

	// Pass len(args) as offset so WHERE placeholders continue correctly.
	whereStr, whereArgs := q.buildWheres(len(args))
	if whereStr != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereStr)
		args = append(args, whereArgs...)
	}

	return sb.String(), args
}

func (q *QueryBuilder[T]) toDeleteSQL() (string, []any) {
	var sb strings.Builder
	sb.WriteString("DELETE FROM ")
	sb.WriteString(q.db.dialect.QuoteIdentifier(q.meta.TableName))

	whereStr, args := q.buildWheres(0)
	if whereStr != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(whereStr)
	}
	return sb.String(), args
}

// ─── Relation Loading ─────────────────────────────────────────────────────────

func (q *QueryBuilder[T]) loadRelation(results []T, name string) error {
	rel := q.getRelation(name)
	if rel == nil {
		return fmt.Errorf("orm: relation %q not found on %s", name, q.meta.TableName)
	}

	switch rel.Type {
	case "has_many":
		return loadHasMany(q.db, results, *rel)
	case "has_one":
		return loadHasOne(q.db, results, *rel)
	case "belongs_to":
		return loadBelongsTo(q.db, results, *rel)
	case "many_to_many":
		return loadManyToMany(q.db, results, *rel)
	case "morph_to":
		return loadMorphTo(q.db, results, *rel)
	case "morph_many":
		return loadMorphMany(q.db, results, *rel)
	}
	return nil
}

// ─── Field Helpers ────────────────────────────────────────────────────────────

// setFieldValue sets a field on v identified by col.FieldIndex.
// No unsafe pointer arithmetic — uses reflect.Value.FieldByIndex.
func setFieldValue(v reflect.Value, col ColumnMeta, val any) {
	field := fieldByIndex(v, col.FieldIndex)
	if !field.CanSet() {
		return
	}
	rv := reflect.ValueOf(val)
	if rv.Type().ConvertibleTo(col.Type) {
		field.Set(rv.Convert(col.Type))
	}
}

// setTimestamp finds a time.Time field by name and sets it (handles embedded structs).
func setTimestamp(v reflect.Value, fieldName string, t time.Time) {
	f := v.FieldByName(fieldName)
	if f.IsValid() && f.CanSet() && f.Type() == reflect.TypeOf(time.Time{}) {
		f.Set(reflect.ValueOf(t))
	}
}
