package models

import "gorm.io/gorm"

// HookType identifies when a hook should fire.
type HookType string

const (
	BeforeCreate HookType = "before_create"
	AfterCreate  HookType = "after_create"
	BeforeSave   HookType = "before_save"
	AfterSave    HookType = "after_save"
	BeforeUpdate HookType = "before_update"
	AfterUpdate  HookType = "after_update"
	BeforeDelete HookType = "before_delete"
	AfterDelete  HookType = "after_delete"
	AfterFind    HookType = "after_find"
)

// ══════════════════════════════════════════════════════════════════════
// Hook Interfaces
//
// Astra uses decorators for hooks:
//   @beforeSave()
//   public static async hashPassword(user: User) { ... }
//
// In Go, we use GORM's hook interfaces. Any model implementing these
// interfaces will have its hooks automatically called by GORM.
//
// Usage:
//   type User struct {
//       models.BaseModel
//       Name     string
//       Password string
//   }
//
//   func (u *User) BeforeSave(tx *gorm.DB) error {
//       if u.Password != "" {
//           hashed, _ := hash.Make(u.Password)
//           u.Password = hashed
//       }
//       return nil
//   }
// ══════════════════════════════════════════════════════════════════════

// BeforeCreateHook is called before creating a record.
// Implement this in your model to run logic before insert.
type BeforeCreateHook interface {
	BeforeCreate(tx *gorm.DB) error
}

// AfterCreateHook is called after creating a record.
type AfterCreateHook interface {
	AfterCreate(tx *gorm.DB) error
}

// BeforeSaveHook is called before saving (create or update).
type BeforeSaveHook interface {
	BeforeSave(tx *gorm.DB) error
}

// AfterSaveHook is called after saving.
type AfterSaveHook interface {
	AfterSave(tx *gorm.DB) error
}

// BeforeUpdateHook is called before updating a record.
type BeforeUpdateHook interface {
	BeforeUpdate(tx *gorm.DB) error
}

// AfterUpdateHook is called after updating.
type AfterUpdateHook interface {
	AfterUpdate(tx *gorm.DB) error
}

// BeforeDeleteHook is called before deleting a record.
type BeforeDeleteHook interface {
	BeforeDelete(tx *gorm.DB) error
}

// AfterDeleteHook is called after deleting.
type AfterDeleteHook interface {
	AfterDelete(tx *gorm.DB) error
}

// AfterFindHook is called after finding/loading a record.
type AfterFindHook interface {
	AfterFind(tx *gorm.DB) error
}

// ══════════════════════════════════════════════════════════════════════
// Relationship Helpers
//
// Astra uses decorators for relationships:
//   @hasMany(() => Post)
//   public posts: HasMany<typeof Post>
//
// In Go, relationships are defined via GORM struct tags:
//   type User struct {
//       models.BaseModel
//       Posts []Post `json:"posts" gorm:"foreignKey:UserID"`
//   }
//
// Eager loading uses the Preload method on the query builder:
//   users, _ := models.Query[User](db).Preload("Posts").All()
// ══════════════════════════════════════════════════════════════════════

// HasOne represents a has-one relationship field tag helper.
// In your model, use: OtherModel OtherType `gorm:"foreignKey:ModelID"`
//
// Example:
//   type User struct {
//       models.BaseModel
//       Profile Profile `json:"profile" gorm:"foreignKey:UserID"`
//   }

// HasMany represents a has-many relationship.
// In your model, use: []OtherModel `gorm:"foreignKey:ModelID"`
//
// Example:
//   type User struct {
//       models.BaseModel
//       Posts []Post `json:"posts" gorm:"foreignKey:UserID"`
//   }

// BelongsTo represents a belongs-to relationship.
// In your model, add the foreign key field and the relation:
//
// Example:
//   type Post struct {
//       models.BaseModel
//       UserID uint `json:"user_id"`
//       User   User `json:"user" gorm:"foreignKey:UserID"`
//   }

// ManyToMany represents a many-to-many relationship.
// Uses GORM's many2many tag:
//
// Example:
//   type User struct {
//       models.BaseModel
//       Roles []Role `json:"roles" gorm:"many2many:user_roles;"`
//   }
