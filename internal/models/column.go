package models

type Column struct {
	Title string
	Path  string
	Cards []Card
}

func NewColumn(title, path string, cards ...Card) Column {
	return Column{
		Title: title,
		Path:  path,
		Cards: cards,
	}
}

func (c Column) CardCount() int {
	return len(c.Cards)
}
