package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/board"
)

type Model struct {
	board board.Board
}

func NewModel(b board.Board) Model {
	return Model{board: b}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return renderBoard(m.board)
}
