package models

type OrderWork struct {
	OrderID uint `gorm:"primaryKey"`
	WorkID  uint `gorm:"primaryKey"`

	Work Work `gorm:"foreignKey:WorkID;constraint:OnDelete:RESTRICT"`

	Quantity int    `gorm:"not null;default:1"`
	Comment  string `gorm:"size:500"`
}
