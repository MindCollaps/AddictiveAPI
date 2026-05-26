package models

import "time"

type Notification struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"not null;index" json:"user_id"`
	Title       string     `gorm:"not null" json:"title"`
	Content     string     `gorm:"not null" json:"content"`
	Style       string     `gorm:"not null;default:info" json:"style"`
	DeliveredAt *time.Time `gorm:"index" json:"delivered_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
