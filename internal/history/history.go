package history

import "kanban/internal/board"

const maxHistorySize = 100

type History struct {
	undoStack []board.Board
	redoStack []board.Board
}

func New() *History {
	return &History{
		undoStack: make([]board.Board, 0, maxHistorySize),
		redoStack: make([]board.Board, 0, maxHistorySize),
	}
}

func (h *History) Push(b board.Board) {
	if len(h.undoStack) >= maxHistorySize {
		h.undoStack = h.undoStack[1:]
	}
	h.undoStack = append(h.undoStack, b.DeepCopy())
	h.redoStack = h.redoStack[:0]
}

func (h *History) Undo(current board.Board) (board.Board, bool) {
	if len(h.undoStack) == 0 {
		return board.Board{}, false
	}

	previousState := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]

	if len(h.redoStack) >= maxHistorySize {
		h.redoStack = h.redoStack[1:]
	}
	h.redoStack = append(h.redoStack, current.DeepCopy())

	return previousState, true
}

func (h *History) Redo(current board.Board) (board.Board, bool) {
	if len(h.redoStack) == 0 {
		return board.Board{}, false
	}

	nextState := h.redoStack[len(h.redoStack)-1]
	h.redoStack = h.redoStack[:len(h.redoStack)-1]

	if len(h.undoStack) >= maxHistorySize {
		h.undoStack = h.undoStack[1:]
	}
	h.undoStack = append(h.undoStack, current.DeepCopy())

	return nextState, true
}

func (h *History) Drop() {
	if len(h.undoStack) > 0 {
		h.undoStack = h.undoStack[:len(h.undoStack)-1]
	}
}
