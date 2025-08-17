package board

import "kanban/internal/column"

type Board struct {
	Columns []column.Column
}

func New(columns []column.Column) Board {
	return Board{Columns: columns}
}
