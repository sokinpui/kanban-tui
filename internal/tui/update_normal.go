// internal/tui/update_normal.go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/card"
	"kanban/internal/fs"
)

func (m *Model) updateNormalMode(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	if keyMsg.Type == tea.KeyCtrlP {
		return m.openFZF()
	}

	switch keyMsg.String() {
	case "q", "ctrl+c":
		if len(m.boardStack) > 0 {
			return m.popBoard()
		}
		return tea.Quit

	case "esc":
		m.selected = make(map[string]struct{})
		m.clipboard = []card.Card{}
		m.isCut = false

	case ":":
		m.statusMessage = ""
		m.mode = commandMode
		m.textInput.SetValue("")
		m.updateCompletions()
		return m.textInput.Focus()

	case "/":
		m.statusMessage = ""
		m.mode = searchMode
		m.textInput.Prompt = "/"
		m.textInput.SetValue("")
		return m.textInput.Focus()

	case "?":
		m.statusMessage = ""
		m.mode = searchMode
		m.textInput.Prompt = "?"
		m.textInput.SetValue("")
		return m.textInput.Focus()

	case "n":
		return m.findNext()

	case "N":
		return m.findPrev()

	case "h", "left":
		if m.focusedColumn > 0 {
			m.focusedColumn--
			m.clampFocusedCard()
			m.ensureFocusedCardIsVisible()
		}

	case "l", "right":
		if m.focusedColumn < len(m.displayColumns)-1 {
			m.focusedColumn++
			m.clampFocusedCard()
			m.ensureFocusedCardIsVisible()
		}

	case "k", "up":
		currentFocus := m.currentFocusedCard()
		if currentFocus > 0 {
			m.setCurrentFocusedCard(currentFocus - 1)
			m.ensureFocusedCardIsVisible()
		}

	case "j", "down":
		currentFocus := m.currentFocusedCard()
		if currentFocus < len(m.displayColumns[m.focusedColumn].Cards) {
			m.setCurrentFocusedCard(currentFocus + 1)
			m.ensureFocusedCardIsVisible()
		}

	case "g":
		if time.Since(m.lastGPress) < 500*time.Millisecond {
			if len(m.displayColumns[m.focusedColumn].Cards) > 0 {
				m.setCurrentFocusedCard(1)
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
			m.ensureFocusedCardIsVisible()
		}

	case "f":
		if time.Since(m.lastGPress) < 500*time.Millisecond { // gf
			m.lastGPress = time.Time{} // Reset timer
			currentFocus := m.currentFocusedCard()
			if currentFocus > 0 {
				crd := m.displayColumns[m.focusedColumn].Cards[currentFocus-1]
				if crd.HasLink() {
					return switchToBoardCmd(crd.Link)
				}
				m.statusMessage = "Card has no link"
				return clearStatusCmd(2 * time.Second)
			}
		}

	case "enter":
		currentFocus := m.currentFocusedCard()
		if currentFocus > 0 {
			cardToEdit := m.displayColumns[m.focusedColumn].Cards[currentFocus-1]
			return openEditor(cardToEdit.Path)
		}

	case "O":
		m.statusMessage = ""
		m.createCardMode = "before"
		m.mode = commandMode
		m.textInput.SetValue("new ")
		return m.textInput.Focus()

	case "o":
		m.statusMessage = ""
		m.createCardMode = "after"
		m.mode = commandMode
		m.textInput.SetValue("new ")
		return m.textInput.Focus()

	case "v", "V":
		currentFocus := m.currentFocusedCard()
		if currentFocus > 0 {
			m.mode = visualMode
			m.visualSelectStart = currentFocus - 1
			m.updateVisualSelection()
		}

	case "y":
		if time.Since(m.lastYPress) < 500*time.Millisecond { // yy
			currentFocus := m.currentFocusedCard()
			if currentFocus > 0 {
				m.clipboard = []card.Card{m.displayColumns[m.focusedColumn].Cards[currentFocus-1]}
				m.isCut = false
				m.selected = make(map[string]struct{})
			}
			m.lastYPress = time.Time{}
		} else {
			m.lastYPress = time.Now()
		}

	case "d":
		if time.Since(m.lastDPress) < 500*time.Millisecond { // dd
			currentFocus := m.currentFocusedCard()
			if currentFocus > 0 {
				m.clipboard = []card.Card{m.displayColumns[m.focusedColumn].Cards[currentFocus-1]}
				m.isCut = true
				m.selected = make(map[string]struct{})
			}
			m.lastDPress = time.Time{}
		} else {
			m.lastDPress = time.Now()
		}

	case "p", "P":
		if len(m.clipboard) == 0 {
			return nil
		}

		m.saveStateForUndo()
		destCol := m.displayColumns[m.focusedColumn]

		insertIndex := 0
		currentFocus := m.currentFocusedCard()
		if currentFocus > 0 {
			if keyMsg.String() == "p" {
				insertIndex = currentFocus
			} else {
				insertIndex = currentFocus - 1
			}
		}

		if insertIndex > len(destCol.Cards) {
			insertIndex = len(destCol.Cards)
		}

		if m.isCut {
			clipboardUUIDs := make(map[string]struct{}, len(m.clipboard))
			for i := range m.clipboard {
				c := &m.clipboard[i]
				clipboardUUIDs[c.UUID] = struct{}{}
				fs.MoveCard(c, *destCol)
			}

			// Remove cut cards from all columns that are NOT the destination.
			for i := range m.board.Columns {
				col := &m.board.Columns[i]
				if col.Title == destCol.Title {
					continue
				}
				keptCards := make([]card.Card, 0, len(col.Cards))
				for _, c := range col.Cards {
					if _, isCut := clipboardUUIDs[c.UUID]; !isCut {
						keptCards = append(keptCards, c)
					}
				}
				col.Cards = keptCards
			}
			if m.board.Archived.Title != destCol.Title {
				keptArchived := make([]card.Card, 0, len(m.board.Archived.Cards))
				for _, c := range m.board.Archived.Cards {
					if _, isCut := clipboardUUIDs[c.UUID]; !isCut {
						keptArchived = append(keptArchived, c)
					}
				}
				m.board.Archived.Cards = keptArchived
			}

			// Rebuild the destination column's card list, inserting the clipboard.
			// This correctly handles cutting and pasting within the same column.
			newDestCards := make([]card.Card, 0, len(destCol.Cards)+len(m.clipboard))

			for i := 0; i < insertIndex; i++ {
				c := destCol.Cards[i]
				if _, isCut := clipboardUUIDs[c.UUID]; !isCut {
					newDestCards = append(newDestCards, c)
				}
			}

			newDestCards = append(newDestCards, m.clipboard...)

			for i := insertIndex; i < len(destCol.Cards); i++ {
				c := destCol.Cards[i]
				if _, isCut := clipboardUUIDs[c.UUID]; !isCut {
					newDestCards = append(newDestCards, c)
				}
			}
			destCol.Cards = newDestCards
		} else {
			var newCards []card.Card
			for _, c := range m.clipboard {
				newCard, err := fs.CopyCard(c, *destCol)
				if err == nil {
					newCards = append(newCards, newCard)
				}
			}
			if len(newCards) > 0 {
				destCol.Cards = append(destCol.Cards[:insertIndex], append(newCards, destCol.Cards[insertIndex:]...)...)
			}
		}
		m.isCut = false
		m.clipboard = []card.Card{}
		m.clampFocusedCard()
		m.ensureFocusedCardIsVisible()

	case "delete":
		var cardsToDelete []card.Card
		if len(m.selected) > 0 {
			for _, col := range m.board.Columns {
				for _, c := range col.Cards {
					if _, isSelected := m.selected[c.UUID]; isSelected {
						cardsToDelete = append(cardsToDelete, c)
					}
				}
			}
		} else if m.currentFocusedCard() > 0 {
			cardIndex := m.currentFocusedCard() - 1
			cardsToDelete = append(cardsToDelete, m.displayColumns[m.focusedColumn].Cards[cardIndex])
		}

		m.saveStateForUndo()
		m.deleteCards(cardsToDelete)
		m.updateDisplayColumns()
		m.clampFocusedCard()

	case "u":
		if newState, ok := m.history.Undo(m.board); ok {
			m.board = newState
			m.updateAndResizeFocus()
			m.statusMessage = "Undo successful"
			return clearStatusCmd(2 * time.Second)
		}
		m.statusMessage = "Nothing to undo"
		return clearStatusCmd(2 * time.Second)

	case "ctrl+r":
		if newState, ok := m.history.Redo(m.board); ok {
			m.board = newState
			m.updateAndResizeFocus()
			m.statusMessage = "Redo successful"
			return clearStatusCmd(2 * time.Second)
		}
		m.statusMessage = "Nothing to redo"
		return clearStatusCmd(2 * time.Second)

	case ".":
		if m.lastCommand != "" {
			m.statusMessage = "Repeating: " + m.lastCommand
			m.ExecuteCommand(m.lastCommand)
			return clearStatusCmd(2 * time.Second)
		}
		m.statusMessage = "No command to repeat"
		return clearStatusCmd(2 * time.Second)
	}
	return nil
}
