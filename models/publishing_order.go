package models

import "time"

type OrderStatus string

const (
	StatusDraft     OrderStatus = "draft"     // черновик
	StatusDeleted   OrderStatus = "deleted"   // удалён
	StatusFormed    OrderStatus = "formed"    // сформирован
	StatusCompleted OrderStatus = "completed" // завершён
	StatusRejected  OrderStatus = "rejected"  // отклонён
)

type PublishingOrder struct {
	// 4 обязательных NotNull поля по требованию
	ID        uint        `gorm:"primaryKey"`
	Status    OrderStatus `gorm:"not null;default:'draft'"`
	CreatedAt time.Time   `gorm:"not null;autoCreateTime"`
	CreatorID uint        `gorm:"not null"`

	// Nullable поля (2 действия создателя + 2 действия модератора)
	FormedAt    *time.Time
	CompletedAt *time.Time
	ModeratorID *uint

	// Связи (без каскадного удаления!)
	Creator   User  `gorm:"foreignKey:CreatorID;constraint:OnDelete:RESTRICT"`
	Moderator *User `gorm:"foreignKey:ModeratorID;constraint:OnDelete:RESTRICT"`

	// Тематические поля издательства
	BookTitle   string `gorm:"size:300"`  // название книги
	Circulation int    `gorm:"default:0"` // тираж (экземпляры)
	TotalPrice  *int   // рассчитывается при формировании заявки

	// Связь с услугами
	Works []OrderWork `gorm:"foreignKey:OrderID"`
}
