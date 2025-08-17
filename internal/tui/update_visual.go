// internal/tui/update_visual.go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/card"
)

func (m *Model) updateVisualMode(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "q", "ctrl+c":
		return tea.Quit

	case "esc", "v", "V":
		m.mode = normalMode
		m.selected = make(map[string]struct{})
		m.visualSelectStart = -1

	case "h", "left", "l", "right":
		m.mode = normalMode
		m.selected = make(map[string]struct{})
		m.visualSelectStart = -1

	case "j", "down":
		currentFocus := m.currentFocusedCard()
		if currentFocus < len(m.board.Columns[m.focusedColumn].Cards) {
			m.setCurrentFocusedCard(currentFocus + 1)
			m.updateVisualSelection()
			m.ensureFocusedCardIsVisible()
		}

	case "k", "up":
		currentFocus := m.currentFocusedCard()
		if currentFocus > 1 { // Stop at the first card
			m.setCurrentFocusedCard(currentFocus - 1)
			m.updateVisualSelection()
			m.ensureFocusedCardIsVisible()
		}

	case "g":
		if time.Since(m.lastGPress) < 500*time.Millisecond {
			if len(m.board.Columns[m.focusedColumn].Cards) > 0 {
				m.setCurrentFocusedCard(1)
				m.updateVisualSelection()
				m.ensureFocusedCardIsVisible()
			}
			m.lastGPress = time.Time{}
		} else {
			m.lastGPress = time.Now()
		}

	case "G":
		numCards := len(m.board.Columns[m.focusedColumn].Cards)
		if numCards > 0 {
			m.setCurrentFocusedCard(numCards)
			m.updateVisualSelection()
			m.ensureFocusedCardIsVisible()
		}

	case "y":
		if len(m.selected) > 0 {
			m.clipboard = []card.Card{}
			m.isCut = false
			for _, c := range m.board.Columns[m.focusedColumn].Cards {
				if _, ok := m.selected[c.UUID]; ok {
					m.clipboard = append(m.clipboard, c)
				}
			}
		}
		m.mode = normalMode
		m.selected = make(map[string]struct{})
		m.visualSelectStart = -1

	case "d":
		if len(m.selected) > 0 {
			m.clipboard = []card.Card{}
			m.isCut = true
			for _, c := range m.board.Columns[m.focusedColumn].Cards {
				if _, ok := m.selected[c.UUID]; ok {
					m.clipboard = append(m.clipboard, c)
				}
			}
		}
		m.mode = normalMode
		m.selected = make(map[string]struct{})
		m.visualSelectStart = -1
	}
	return nil
}

func (m *Model) updateVisualSelection() {
	m.selected = make(map[string]struct{})
	if m.visualSelectStart == -1 {
		return
	}

	currentFocusIdx := m.currentFocusedCard() - 1
	if currentFocusIdx < 0 {
		return
	}

	start, end := m.visualSelectStart, currentFocusIdx
	if start > end {
		start, end = end, start
	}

	cards := m.board.Columns[m.focusedColumn].Cards
	for i := start; i <= end; i++ {
		if i < len(cards) {
			m.selected[cards[i].UUID] = struct{}{}
		}
	}
}
