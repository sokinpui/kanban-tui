package card

import "time"

type Card struct {
	UUID       string    `yaml:"-"` // Derived from filename
	Path       string    `yaml:"-"`
	Title      string    `yaml:"title"`
	Content    string    `yaml:"-"`
	CreatedAt  time.Time `yaml:"createdAt"`
	ModifiedAt time.Time `yaml:"modifiedAt"`
}

func New(title string) Card {
	now := time.Now()
	return Card{Title: title, CreatedAt: now, ModifiedAt: now}
}

func (c Card) HasContent() bool {
	return c.Content != ""
}
