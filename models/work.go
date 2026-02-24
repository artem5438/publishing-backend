package models

type WorkStatus string

const (
	WorkStatusActive  WorkStatus = "active"
	WorkStatusDeleted WorkStatus = "deleted"
)

type Work struct {
	ID          uint       `gorm:"primaryKey"`
	Name        string     `gorm:"not null;size:200"`
	Description string     `gorm:"type:text"`
	Status      WorkStatus `gorm:"not null;default:'active'"`
	ImageKey    *string    `gorm:"size:255"`
	VideoKey    *string    `gorm:"size:255"`
	PriceRub    int        `gorm:"not null"`

	// Тематические поля
	WorkType string `gorm:"not null;size:100"`
	Unit     string `gorm:"not null;size:50"`

	// Params — строки как были в лабе 1
	ParamDeadline string `gorm:"size:100"`
	ParamQuantity string `gorm:"size:100"`
	ParamUnit     string `gorm:"size:100"`
	ParamFormat   string `gorm:"size:100"`

	// Tags — три отдельных поля (1НФ — массивы запрещены!)
	Tag1 string `gorm:"size:100"`
	Tag2 string `gorm:"size:100"`
	Tag3 string `gorm:"size:100"`
}
