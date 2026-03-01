// Package models provides the Lucid ORM — an Active Record implementation
// for Go that mirrors Astra's Lucid ORM.
//
// Go Idiom Note: Astra Lucid uses class-based models with static methods:
//
//	const user = await User.find(1)
//	await user.save()
//
// In Go, we cannot have static methods on structs. Instead, we use a generic
// Model[T] wrapper that provides all ORM operations. Usage:
//
//	user, err := models.Find[User](db, 1)
//	user.Name = "John"
//	err = models.Save(db, &user)
//
// For query builder:
//
//	users, err := models.Query[User](db).Where("age > ?", 18).OrderBy("name", "asc").All()
package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel provides common fields for all models.
// Embed this in your model structs to get timestamps and soft deletes.
// Mirrors Astra's BaseModel class.
//
// Usage:
//
//	type User struct {
//	    models.BaseModel
//	    Name  string `json:"name" gorm:"not null"`
//	    Email string `json:"email" gorm:"uniqueIndex;not null"`
//	}
type BaseModel struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// GetID returns the primary key value.
func (m *BaseModel) GetID() any { return m.ID }

// PrimaryKey returns the primary key column name.
func (m *BaseModel) PrimaryKey() string { return "id" }

// GetCreatedAt returns the creation timestamp.
func (m *BaseModel) GetCreatedAt() time.Time { return m.CreatedAt }

// GetUpdatedAt returns the last update timestamp.
func (m *BaseModel) GetUpdatedAt() time.Time { return m.UpdatedAt }

// IsPersisted returns true if the model has been saved to the database.
func (m *BaseModel) IsPersisted() bool { return m.ID != 0 }

// ══════════════════════════════════════════════════════════════════════
// Static Model Methods (Generic Functions)
// These replicate Astra's static model methods like User.find(1)
// ══════════════════════════════════════════════════════════════════════

// Find retrieves a single record by primary key.
// Mirrors: const user = await User.find(1)
func Find[T any](db *gorm.DB, id any) (*T, error) {
	var model T
	result := db.First(&model, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

// FindOrFail retrieves a record by primary key or returns an error.
// Mirrors: const user = await User.findOrFail(1)
func FindOrFail[T any](db *gorm.DB, id any) (*T, error) {
	model, err := Find[T](db, id)
	if err != nil {
		return nil, err
	}
	return model, nil
}

// FindBy retrieves a single record matching the given column/value.
// Mirrors: const user = await User.findBy('email', 'test@example.com')
func FindBy[T any](db *gorm.DB, column string, value any) (*T, error) {
	var model T
	result := db.Where(column+" = ?", value).First(&model)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

// All retrieves all records.
// Mirrors: const users = await User.all()
func All[T any](db *gorm.DB) ([]T, error) {
	var models []T
	result := db.Find(&models)
	if result.Error != nil {
		return nil, result.Error
	}
	return models, nil
}

// Create creates a new record in the database.
// Mirrors: const user = await User.create({ name: 'John', email: 'john@example.com' })
func Create[T any](db *gorm.DB, model *T) error {
	return db.Create(model).Error
}

// Save saves (insert or update) a model instance.
// Mirrors: await user.save()
func Save[T any](db *gorm.DB, model *T) error {
	return db.Save(model).Error
}

// Delete deletes a model instance.
// Mirrors: await user.delete()
func Delete[T any](db *gorm.DB, model *T) error {
	return db.Delete(model).Error
}

// UpdateOrCreate finds the first record matching conditions, or creates a new one.
// Mirrors: const user = await User.updateOrCreate({ email }, { name, email })
func UpdateOrCreate[T any](db *gorm.DB, conditions map[string]any, values map[string]any) (*T, error) {
	var model T
	result := db.Where(conditions).First(&model)
	if result.Error != nil {
		// Not found, create
		for k, v := range conditions {
			values[k] = v
		}
		result = db.Model(&model).Create(values)
		if result.Error != nil {
			return nil, result.Error
		}
		// Re-fetch to get the complete model
		db.Where(conditions).First(&model)
		return &model, nil
	}
	// Found, update
	result = db.Model(&model).Updates(values)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

// FirstOrCreate finds the first record matching conditions, or creates a new one.
// Mirrors: const user = await User.firstOrCreate({ email }, { name, email })
func FirstOrCreate[T any](db *gorm.DB, conditions *T, defaults *T) (*T, error) {
	var model T
	result := db.Where(conditions).FirstOrCreate(&model, defaults)
	if result.Error != nil {
		return nil, result.Error
	}
	return &model, nil
}

// Count returns the count of records.
func Count[T any](db *gorm.DB) (int64, error) {
	var model T
	var count int64
	result := db.Model(&model).Count(&count)
	return count, result.Error
}

// Truncate deletes all records from the table (for seeding/testing).
func Truncate[T any](db *gorm.DB) error {
	var model T
	return db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&model).Error
}
