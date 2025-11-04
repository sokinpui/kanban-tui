package history

import "kanban/internal/models"

const maxHistorySize = 100

type History struct {
	undoStack []models.Board
	redoStack []models.Board
}

func New() *History {
	return &History{
		undoStack: make([]models.Board, 0, maxHistorySize),
		redoStack: make([]models.Board, 0, maxHistorySize),
	}
}

func (h *History) Push(b models.Board) {
	if len(h.undoStack) >= maxHistorySize {
		h.undoStack = h.undoStack[1:]
	}
	h.undoStack = append(h.undoStack, b.DeepCopy())
	h.redoStack = h.redoStack[:0]
}

func (h *History) Undo(current models.Board) (models.Board, bool) {
	if len(h.undoStack) == 0 {
		return models.Board{}, false
	}

	previousState := h.undoStack[len(h.undoStack)-1]
	h.undoStack = h.undoStack[:len(h.undoStack)-1]

	if len(h.redoStack) >= maxHistorySize {
		h.redoStack = h.redoStack[1:]
	}
	h.redoStack = append(h.redoStack, current.DeepCopy())

	return previousState, true
}

func (h *History) Redo(current models.Board) (models.Board, bool) {
	if len(h.redoStack) == 0 {
		return models.Board{}, false
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
