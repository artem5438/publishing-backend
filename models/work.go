package models

type WorkStatus string

const (
	WorkStatusActive  WorkStatus = "active"
	WorkStatusDeleted WorkStatus = "deleted"
)

type Work struct {
	ID           uint       `gorm:"primaryKey"`
	Name         string     `gorm:"not null;size:200"`
	Description  string     `gorm:"type:text"`
	Status       WorkStatus `gorm:"not null;default:'active'"`
	ImageKey     *string    `gorm:"size:255"` // Nullable
	VideoKey     *string    `gorm:"size:255"` // Nullable
	PriceRub     int        `gorm:"not null"`
	WorkType     string     `gorm:"not null;size:100"`  // печать / верстка / переплёт ...
	LeadTimeDays int        `gorm:"not null;default:1"` // срок в рабочих днях
	Unit         string     `gorm:"not null;size:50"`   // экз. / стр. / шт.
}
