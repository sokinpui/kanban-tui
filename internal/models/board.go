package models

type Board struct {
	// Path is the root directory containing kanban.md
	Path     string
	Columns  []Column
	Archived Column
	Trash    []Card
}

func New(path string, columns []Column) Board {
	return Board{Path: path, Columns: columns, Trash: []Card{}}
}

func (b *Board) DeepCopy() Board {
	newBoard := Board{
		Path: b.Path,
	}

	newBoard.Columns = make([]Column, len(b.Columns))
	for i, col := range b.Columns {
		newCol := Column{Title: col.Title, Path: col.Path}
		newCol.Cards = make([]Card, len(col.Cards))
		copy(newCol.Cards, col.Cards)
		newBoard.Columns[i] = newCol
	}

	newArchived := Column{
		Title: b.Archived.Title,
		Path:  b.Archived.Path,
	}
	newArchived.Cards = make([]Card, len(b.Archived.Cards))
	copy(newArchived.Cards, b.Archived.Cards)
	newBoard.Archived = newArchived

	newBoard.Trash = make([]Card, len(b.Trash))
	copy(newBoard.Trash, b.Trash)

	return newBoard
}
