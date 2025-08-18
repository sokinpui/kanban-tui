package history

import "kanban/internal/board"

const maxHistorySize = 100

type History struct {
	states []board.Board
}

func New() *History {
	return &History{
		states: make([]board.Board, 0),
	}
}

func (h *History) Push(b board.Board) {
	if len(h.states) >= maxHistorySize {
		h.states = h.states[1:]
	}
	h.states = append(h.states, b.DeepCopy())
}

func (h *History) Pop() (board.Board, bool) {
	if len(h.states) == 0 {
		return board.Board{}, false
	}
	lastState := h.states[len(h.states)-1]
	h.states = h.states[:len(h.states)-1]
	return lastState, true
}
