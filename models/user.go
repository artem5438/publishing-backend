package models

import "time"

type UserRole string

const (
	RoleCreator   UserRole = "creator"
	RoleModerator UserRole = "moderator"
)

type User struct {
	ID        uint     `gorm:"primaryKey"`
	Login     string   `gorm:"uniqueIndex;not null;size:100"`
	Password  string   `gorm:"not null;size:255"`
	Name      string   `gorm:"not null;size:200"`
	Role      UserRole `gorm:"not null;default:'creator'"`
	CreatedAt time.Time
}
