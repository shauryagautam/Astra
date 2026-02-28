package main

import (
	"time"

	models "github.com/shaurya/adonis/app/Models"
)

// User model
type User struct {
	models.BaseModel
	Name     string `gorm:"size:255;not null" json:"name"`
	Email    string `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password string `gorm:"size:255;not null" json:"-"`
	Age      int    `json:"age"`
}

// Post model
type Post struct {
	models.BaseModel
	Title       string     `gorm:"size:255;not null" json:"title"`
	Content     string     `gorm:"type:text" json:"content"`
	UserID      uint       `json:"user_id"`
	PublishedAt *time.Time `json:"published_at"`
}
