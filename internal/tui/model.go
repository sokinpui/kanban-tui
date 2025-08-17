package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/board"
)

type mode int

const (
	normalMode mode = iota
	insertMode
)

const defaultCardWidth = 22

type Model struct {
	board         board.Board
	focusedColumn int
	focusedCard   int
	mode          mode
	textInput     textinput.Model
}

func NewModel(b board.Board) Model {
	ti := textinput.New()
	ti.Placeholder = "Card title"
	ti.CharLimit = 156
	ti.Width = defaultCardWidth

	return Model{
		board:         b,
		focusedColumn: 0,
		focusedCard:   0,
		mode:          normalMode,
		textInput:     ti,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, nil

	case tea.KeyMsg:
		if m.mode == insertMode {
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				if len(m.board.Columns[m.focusedColumn].Cards) > 0 {
					m.board.Columns[m.focusedColumn].Cards[m.focusedCard].Title = m.textInput.Value()
				}
				m.textInput.Blur()
				m.textInput.Width = defaultCardWidth
				return m, nil
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
		}

		// Normal Mode
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "h", "left":
			if m.focusedColumn > 0 {
				m.focusedColumn--
				// Clamp focused card to the new column's card count
				if m.focusedCard >= len(m.board.Columns[m.focusedColumn].Cards) {
					m.focusedCard = len(m.board.Columns[m.focusedColumn].Cards) - 1
					if m.focusedCard < 0 {
						m.focusedCard = 0
					}
				}
			}

		case "l", "right":
			if m.focusedColumn < len(m.board.Columns)-1 {
				m.focusedColumn++
				// Clamp focused card to the new column's card count
				if m.focusedCard >= len(m.board.Columns[m.focusedColumn].Cards) {
					m.focusedCard = len(m.board.Columns[m.focusedColumn].Cards) - 1
					if m.focusedCard < 0 {
						m.focusedCard = 0
					}
				}
			}

		case "k", "up":
			if len(m.board.Columns[m.focusedColumn].Cards) > 0 && m.focusedCard > 0 {
				m.focusedCard--
			}

		case "j", "down":
			if len(m.board.Columns[m.focusedColumn].Cards) > 0 && m.focusedCard < len(m.board.Columns[m.focusedColumn].Cards)-1 {
				m.focusedCard++
			}

		case "i":
			if len(m.board.Columns[m.focusedColumn].Cards) > 0 {
				m.mode = insertMode
				currentTitle := m.board.Columns[m.focusedColumn].Cards[m.focusedCard].Title
				m.textInput.SetValue(currentTitle)

				newWidth := len(currentTitle)
				if newWidth < defaultCardWidth {
					newWidth = defaultCardWidth
				}
				m.textInput.Width = newWidth

				return m, m.textInput.Focus()
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	return renderBoard(m)
}
