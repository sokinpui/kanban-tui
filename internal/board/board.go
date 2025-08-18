package board

import (
	"kanban/internal/card"
	"kanban/internal/column"
)

type Board struct {
	// Path is the root directory containing kanban.md
	Path    string
	Columns []column.Column
	Archived column.Column
}

func New(path string, columns []column.Column) Board {
	return Board{Path: path, Columns: columns}
}

func (b *Board) DeepCopy() Board {
	newBoard := Board{
		Path: b.Path,
	}

	newBoard.Columns = make([]column.Column, len(b.Columns))
	for i, col := range b.Columns {
		newCol := column.Column{Title: col.Title, Path: col.Path}
		newCol.Cards = make([]card.Card, len(col.Cards))
		copy(newCol.Cards, col.Cards)
		newBoard.Columns[i] = newCol
	}

	newArchived := column.Column{
		Title: b.Archived.Title,
		Path:  b.Archived.Path,
	}
	newArchived.Cards = make([]card.Card, len(b.Archived.Cards))
	copy(newArchived.Cards, b.Archived.Cards)
	newBoard.Archived = newArchived

	return newBoard
}
