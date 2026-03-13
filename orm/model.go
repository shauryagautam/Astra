package orm

import "time"

// Model is the base struct embedded in all models.
type Model struct {
	ID        uint       `orm:"column:id;primaryKey;autoIncrement"`
	CreatedAt time.Time  `orm:"column:created_at"`
	UpdatedAt time.Time  `orm:"column:updated_at"`
	DeletedAt *time.Time `orm:"column:deleted_at;soft_delete"`
}

// TableNamer allows models to override their table name
type TableNamer interface {
	TableName() string
}

// Relation types for eager loading
type HasMany[T any] struct {
	loaded bool
	items  []T
}

type HasOne[T any] struct {
	loaded bool
	item   *T
}

type BelongsTo[T any] struct {
	loaded bool
	item   *T
}

type ManyToMany[T any] struct {
	loaded bool
	items  []T
	pivot  string
}

type MorphTo[T any] struct {
	loaded bool
	item   *T
	id     any
	typ    string
}

type MorphMany[T any] struct {
	loaded bool
	items  []T
}

// Getter methods for relations
func (h *HasMany[T]) Get() []T    { return h.items }
func (h *HasOne[T]) Get() *T      { return h.item }
func (b *BelongsTo[T]) Get() *T   { return b.item }
func (m *ManyToMany[T]) Get() []T { return m.items }
func (m *MorphTo[T]) Get() *T     { return m.item }
func (m *MorphMany[T]) Get() []T  { return m.items }

func (h *HasMany[T]) Loaded() bool    { return h.loaded }
func (h *HasOne[T]) Loaded() bool     { return h.loaded }
func (b *BelongsTo[T]) Loaded() bool  { return b.loaded }
func (m *ManyToMany[T]) Loaded() bool { return m.loaded }
func (m *MorphTo[T]) Loaded() bool    { return m.loaded }
func (m *MorphMany[T]) Loaded() bool  { return m.loaded }

// Attachment represents a file attached to a model.
type Attachment struct {
	Path     string `json:"path" orm:"column:path"`
	Filename string `json:"filename" orm:"column:filename"`
	Size     int64  `json:"size" orm:"column:size"`
	MimeType string `json:"mime_type" orm:"column:mime_type"`
	Disk     string `json:"disk" orm:"column:disk"`
}

// URL returns the public URL for the attachment.
func (a *Attachment) URL() string {
	if AttachmentResolver != nil {
		url, err := AttachmentResolver(a.Disk, a.Path)
		if err == nil {
			return url
		}
	}
	if a.Path == "" {
		return ""
	}
	return "/storage/" + a.Path
}

// HasAttachment is a trait for models that have a single file attachment.
type HasAttachment struct {
	Attachment Attachment `json:"attachment" orm:"prefix:attachment_"`
}
