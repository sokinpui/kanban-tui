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
		if currentFocus < len(m.displayColumns[m.focusedColumn].Cards) {
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
			if len(m.displayColumns[m.focusedColumn].Cards) > 0 {
				m.setCurrentFocusedCard(1)
				m.updateVisualSelection()
				m.ensureFocusedCardIsVisible()
			}
			m.lastGPress = time.Time{}
		} else {
			m.lastGPress = time.Now()
		}

	case "G":
		numCards := len(m.displayColumns[m.focusedColumn].Cards)
		if numCards > 0 {
			m.setCurrentFocusedCard(numCards)
			m.updateVisualSelection()
			m.ensureFocusedCardIsVisible()
		}

	case ":":
		m.statusMessage = ""
		m.mode = commandMode
		m.textInput.SetValue("")
		return m.textInput.Focus()

	case "y":
		if len(m.selected) > 0 {
			m.clipboard = []card.Card{}
			m.isCut = false
			for _, c := range m.displayColumns[m.focusedColumn].Cards {
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
			for _, c := range m.displayColumns[m.focusedColumn].Cards {
				if _, ok := m.selected[c.UUID]; ok {
					m.clipboard = append(m.clipboard, c)
				}
			}
		}
		m.mode = normalMode
		m.selected = make(map[string]struct{})
		m.visualSelectStart = -1

	case "delete", "backspace":
		var cardsToDelete []card.Card
		if len(m.selected) > 0 {
			for _, c := range m.displayColumns[m.focusedColumn].Cards {
				if _, isSelected := m.selected[c.UUID]; isSelected {
					cardsToDelete = append(cardsToDelete, c)
				}
			}
		}
		m.saveStateForUndo()
		m.deleteCards(cardsToDelete)
		m.updateDisplayColumns()
		m.clampFocusedCard()
		m.mode = normalMode
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

	cards := m.displayColumns[m.focusedColumn].Cards
	for i := start; i <= end; i++ {
		if i < len(cards) {
			m.selected[cards[i].UUID] = struct{}{}
		}
	}
}
