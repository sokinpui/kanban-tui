package board

import "kanban/internal/column"

type Board struct {
	// Path is the root directory containing kanban.md
	Path    string
	Columns []column.Column
	Archived column.Column
}

func New(path string, columns []column.Column) Board {
	return Board{Path: path, Columns: columns}
}
