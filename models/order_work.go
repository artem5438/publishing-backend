package models

type OrderWork struct {
	// Составной уникальный ключ (требование курса)
	OrderID uint `gorm:"primaryKey"`
	WorkID  uint `gorm:"primaryKey"`

	// Без каскадного удаления!
	Work Work `gorm:"foreignKey:WorkID;constraint:OnDelete:RESTRICT"`

	// Поля м-м
	Quantity int    `gorm:"not null;default:1"` // количество экземпляров
	Comment  string `gorm:"size:500"`           // комментарий к услуге
}
