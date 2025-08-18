// internal/tui/update_command.go
package tui

import (
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/card"
	"kanban/internal/fs"
)

func (m *Model) updateCommandMode(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.mode = normalMode
			m.textInput.Blur()
			m.createCardMode = "prepend"
			m.selected = make(map[string]struct{})
			m.clipboard = []card.Card{}
			m.isCut = false
			return nil
		case tea.KeyEnter:
			cmd = m.executeCommand()
			m.mode = normalMode
			m.textInput.Blur()
			return cmd
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return cmd
}

func (m *Model) executeCommand() tea.Cmd {
	parts := strings.SplitN(strings.TrimSpace(m.textInput.Value()), " ", 2)
	command := parts[0]

	var args string
	if len(parts) > 1 {
		args = parts[1]
	}

	switch command {
	case "new":
		title := args
		currentCol := &m.board.Columns[m.focusedColumn]

		newCard, err := fs.CreateCard(*currentCol, title)
		if err != nil {
			return nil
		}

		insertIndex := 0
		currentFocus := m.currentFocusedCard()
		switch m.createCardMode {
		case "prepend":
			insertIndex = 0
		case "before":
			if currentFocus > 0 {
				insertIndex = currentFocus - 1
			}
		case "after":
			if currentFocus > 0 {
				insertIndex = currentFocus
			} else {
				insertIndex = 0
			}
		}
		m.createCardMode = "prepend"

		if insertIndex > len(currentCol.Cards) {
			insertIndex = len(currentCol.Cards)
		}

		currentCol.Cards = append(currentCol.Cards[:insertIndex], append([]card.Card{newCard}, currentCol.Cards[insertIndex:]...)...)

		if err := fs.WriteBoard(m.board); err != nil {
			return nil
		}
		m.setCurrentFocusedCard(insertIndex + 1)
		m.ensureFocusedCardIsVisible()
	case "sort":
		sortArgs := strings.Split(args, " ")
		if len(sortArgs) != 2 {
			return nil
		}
		field, direction := sortArgs[0], sortArgs[1]

		currentCol := &m.board.Columns[m.focusedColumn]
		if len(currentCol.Cards) < 2 {
			return nil
		}

		sort.Slice(currentCol.Cards, func(i, j int) bool {
			var timeI, timeJ time.Time
			switch field {
			case "modify":
				timeI = currentCol.Cards[i].ModifiedAt
				timeJ = currentCol.Cards[j].ModifiedAt
			case "create":
				timeI = currentCol.Cards[i].CreatedAt
				timeJ = currentCol.Cards[j].CreatedAt
			default:
				return false
			}

			if direction == "asc" {
				return timeI.Before(timeJ)
			}
			if direction == "desc" {
				return timeI.After(timeJ)
			}
			return false
		})

		fs.WriteBoard(m.board)
		m.setCurrentFocusedCard(0)
		m.ensureFocusedCardIsVisible()
	case "create":
		name := args
		if name == "" {
			return nil
		}
		newCol, err := fs.CreateColumn(name)
		if err != nil {
			// TODO: Show error to user
			return nil
		}
		m.board.Columns = append(m.board.Columns, newCol)
		m.columnCardFocus = append(m.columnCardFocus, 0)
		fs.WriteBoard(m.board)
		m.focusedColumn = len(m.board.Columns) - 1

	case "delete":
		if m.currentFocusedCard() != 0 || len(m.board.Columns) == 0 {
			return nil
		}

		colToDelete := m.board.Columns[m.focusedColumn]
		if err := fs.DeleteColumn(colToDelete); err != nil {
			// TODO: Show error to user
			return nil
		}

		// Remove column from board
		m.board.Columns = append(m.board.Columns[:m.focusedColumn], m.board.Columns[m.focusedColumn+1:]...)
		// Remove focus state for that column
		m.columnCardFocus = append(m.columnCardFocus[:m.focusedColumn], m.columnCardFocus[m.focusedColumn+1:]...)

		// Adjust focus
		if m.focusedColumn >= len(m.board.Columns) {
			m.focusedColumn = len(m.board.Columns) - 1
		}
		if m.focusedColumn < 0 {
			m.focusedColumn = 0
		}
		fs.WriteBoard(m.board)
	case "archive":
		if len(m.selected) == 0 {
			return nil
		}

		if _, err := os.Stat(m.board.Archived.Path); os.IsNotExist(err) {
			if err := os.MkdirAll(m.board.Archived.Path, 0755); err != nil {
				// TODO: show error
				return nil
			}
		}

		cardsToArchive := make([]*card.Card, 0)
		for i := range m.board.Columns {
			for j := range m.board.Columns[i].Cards {
				c := &m.board.Columns[i].Cards[j]
				if _, isSelected := m.selected[c.UUID]; isSelected {
					cardsToArchive = append(cardsToArchive, c)
				}
			}
		}

		movedCards := make([]card.Card, 0, len(cardsToArchive))
		for _, c := range cardsToArchive {
			err := fs.MoveCard(c, m.board.Archived)
			if err == nil {
				movedCards = append(movedCards, *c)
			}
		}
		m.board.Archived.Cards = append(m.board.Archived.Cards, movedCards...)

		for i := range m.board.Columns {
			col := &m.board.Columns[i]
			keptCards := col.Cards[:0]
			for _, c := range col.Cards {
				if _, isSelected := m.selected[c.UUID]; !isSelected {
					keptCards = append(keptCards, c)
				}
			}
			col.Cards = keptCards
		}

		fs.WriteBoard(m.board)

		m.selected = make(map[string]struct{})
		m.clipboard = []card.Card{}
		m.isCut = false
		m.visualSelectStart = -1
		m.clampFocusedCard()
	}

	return nil
}
