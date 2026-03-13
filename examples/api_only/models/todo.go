package models

import "time"

// Todo is our database model
type Todo struct {
	ID        uint      `json:"id" orm:"column:id"`
	Title     string    `json:"title" orm:"column:title"`
	Completed bool      `json:"completed" orm:"column:completed"`
	CreatedAt time.Time `json:"created_at" orm:"column:created_at"`
	UpdatedAt time.Time `json:"updated_at" orm:"column:updated_at"`
}
