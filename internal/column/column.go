package column

import "kanban/internal/card"

type Column struct {
	Title string
	Path  string
	Cards []card.Card
}

func New(title, path string, cards ...card.Card) Column {
	return Column{
		Title: title,
		Path:  path,
		Cards: cards,
	}
}

func (c Column) CardCount() int {
	return len(c.Cards)
}
