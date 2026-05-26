package models

import "time"

type User struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Email         string    `gorm:"uniqueIndex;not null" json:"email"`
	Username      string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash  string    `gorm:"not null" json:"-"`
	Score         int64     `gorm:"not null;default:0" json:"score"`
	ProfilePublic bool      `gorm:"not null;default:false" json:"profile_public"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
