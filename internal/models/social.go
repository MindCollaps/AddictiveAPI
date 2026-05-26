package models

import "time"

type FriendRequest struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RequesterID uint      `gorm:"not null;index:idx_friend_request_pair,unique" json:"requester_id"`
	AddresseeID uint      `gorm:"not null;index:idx_friend_request_pair,unique" json:"addressee_id"`
	Status      string    `gorm:"not null;index" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Follow struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FollowerID uint      `gorm:"not null;index:idx_follow_pair,unique" json:"follower_id"`
	FolloweeID uint      `gorm:"not null;index:idx_follow_pair,unique" json:"followee_id"`
	CreatedAt  time.Time `json:"created_at"`
}
