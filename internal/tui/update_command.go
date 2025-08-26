// internal/tui/update_command.go
package tui

import (
	"os"
	"sort"
	"strings"
	"time"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/card"
	"kanban/internal/column"
	"kanban/internal/fs"
)

func (m *Model) updateCommandMode(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	prevVal := m.textInput.Value()

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.mode = normalMode
			m.textInput.Blur()
			m.createCardMode = "prepend"
			m.selected = make(map[string]struct{})
			m.clipboard = []card.Card{}
			m.isCut = false
			m.completionMatches = nil
			m.completionIndex = -1
			return nil
		case tea.KeyEnter:
			cmd = m.executeUserCommand()
			if m.mode == commandMode {
				m.mode = normalMode
			}
			m.textInput.Blur()
			m.completionMatches = nil
			m.completionIndex = -1
			return cmd
		case tea.KeyTab:
			m.cycleCompletion()
			return nil
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	if m.textInput.Value() != prevVal {
		m.updateCompletions()
	}
	return cmd
}

func (m *Model) updateCompletions() {
	inputValue := m.textInput.Value()
	parts := strings.Split(inputValue, " ")

	if len(parts) == 0 {
		m.completionMatches = nil
		m.completionIndex = -1
		return
	}

	var candidates []string
	wordToComplete := parts[len(parts)-1]
	isCompletingArgument := len(parts) > 1 || (len(parts) == 1 && strings.HasSuffix(inputValue, " "))

	if !isCompletingArgument {
		candidates = []string{
			"archive", "create", "delete", "done", "fzf", "hide", "left", "new",
			"noh", "nohlsearch", "rename", "right", "set", "show", "sort", "sort!",
			"unset",
		}
	} else {
		// If the last char is a space, we are completing the *next* word
		if strings.HasSuffix(inputValue, " ") {
			wordToComplete = ""
			parts = append(parts, "")
		}

		command := parts[0]
		if strings.HasSuffix(command, "!") {
			command = strings.TrimSuffix(command, "!")
		}
		switch command {
		case "sort":
			if len(parts) == 2 {
				candidates = []string{"create", "modify", "name", "size"}
			}
		case "set":
			candidates = []string{"done", "done?"}
		case "unset":
			candidates = []string{"done"}
		case "show", "hide":
			candidates = []string{"hidden"}
		}
	}

	if len(candidates) == 0 {
		m.completionMatches = nil
		m.completionIndex = -1
		return
	}

	var matches []string
	for _, c := range candidates {
		if strings.HasPrefix(c, strings.ToLower(wordToComplete)) {
			matches = append(matches, c)
		}
	}

	if len(matches) == 0 {
		m.completionMatches = nil
		m.completionIndex = -1
		return
	}

	m.completionMatches = matches
	m.completionIndex = -1
}

func (m *Model) cycleCompletion() {
	if len(m.completionMatches) == 0 {
		return
	}

	m.completionIndex++
	if m.completionIndex >= len(m.completionMatches) {
		m.completionIndex = 0
	}

	nextMatch := m.completionMatches[m.completionIndex]

	inputValue := m.textInput.Value()
	parts := strings.Split(inputValue, " ")
	prefixParts := parts[:len(parts)-1]
	newValue := strings.Join(append(prefixParts, nextMatch), " ")

	m.textInput.SetValue(newValue)
	m.textInput.SetCursor(len(newValue))
}

func (m *Model) executeUserCommand() tea.Cmd {
	commandStr := m.textInput.Value()
	m.lastCommand = commandStr
	return m.executeCommand(commandStr)
}

func (m *Model) executeCommand(commandStr string) tea.Cmd {
	parts := strings.SplitN(strings.TrimSpace(commandStr), " ", 2)
	command := parts[0]

	var args string
	if len(parts) > 1 {
		args = parts[1]
	}

	descending := false
	if strings.HasSuffix(command, "!") {
		command = strings.TrimSuffix(command, "!")
		descending = true
	}

	m.saveStateForUndo()
	switch command {
	case "fzf":
		m.history.Drop()
		return m.openFZF()

	case "new":
		title := args
		currentCol := m.displayColumns[m.focusedColumn]

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
		field := "name"
		if args != "" {
			field = args
		}

		validFields := map[string]bool{"name": true, "create": true, "modify": true, "size": true}
		if !validFields[field] {
			m.history.Drop()
			m.statusMessage = fmt.Sprintf("Invalid sort field: %s. Valid: name, create, modify, size.", field)
			return clearStatusCmd(5 * time.Second)
		}

		currentCol := m.displayColumns[m.focusedColumn]
		if len(currentCol.Cards) < 2 {
			return nil
		}

		sort.Slice(currentCol.Cards, func(i, j int) bool {
			cardI := currentCol.Cards[i]
			cardJ := currentCol.Cards[j]
			var less bool

			switch field {
			case "modify":
				less = cardI.ModifiedAt.Before(cardJ.ModifiedAt)
			case "create":
				less = cardI.CreatedAt.Before(cardJ.CreatedAt)
			case "size":
				less = cardI.Size < cardJ.Size
			case "name":
				less = strings.ToLower(cardI.Title) < strings.ToLower(cardJ.Title)
			}

			if descending {
				return !less
			}
			return less
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
		m.updateAndResizeFocus()
		fs.WriteBoard(m.board)
		m.focusedColumn = len(m.displayColumns) - 1

	case "rename":
		newName := strings.TrimSpace(args)
		if newName == "" {
			m.history.Drop()
			m.statusMessage = "Usage: :rename <new-column-name>"
			return clearStatusCmd(3 * time.Second)
		}

		for _, c := range m.board.Columns {
			if c.Title == newName {
				m.history.Drop()
				m.statusMessage = fmt.Sprintf("Column '%s' already exists", newName)
				return clearStatusCmd(3 * time.Second)
			}
		}
		if m.board.Archived.Title == newName {
			m.history.Drop()
			m.statusMessage = fmt.Sprintf("Column '%s' already exists (Archived)", newName)
			return clearStatusCmd(3 * time.Second)
		}

		colToRename := m.displayColumns[m.focusedColumn]
		oldName := colToRename.Title

		if oldName == fs.ArchiveColumnName {
			m.history.Drop()
			m.statusMessage = "Cannot rename the Archived column"
			return clearStatusCmd(3 * time.Second)
		}

		if err := fs.RenameColumn(colToRename, newName); err != nil {
			m.history.Drop()
			m.statusMessage = fmt.Sprintf("Error renaming column: %v", err)
			return clearStatusCmd(5 * time.Second)
		}

		if m.doneColumnName == oldName {
			m.doneColumnName = newName
		}

		fs.WriteBoard(m.board)
		m.statusMessage = fmt.Sprintf("Renamed column '%s' to '%s'", oldName, newName)
		return clearStatusCmd(3 * time.Second)

	case "delete":
		if m.currentFocusedCard() != 0 || len(m.displayColumns) == 0 {
			return nil
		}

		colToDelete := m.board.Columns[m.focusedColumn]
		if err := fs.DeleteColumn(colToDelete); err != nil {
			// TODO: Show error to user
			return nil
		}

		if colToDelete.Title != fs.ArchiveColumnName {
			var newCols []column.Column
			for _, c := range m.board.Columns {
				if c.Title != colToDelete.Title {
					newCols = append(newCols, c)
				}
			}
			m.board.Columns = newCols
		} else {
			// It's the archive column, just clear it
			m.board.Archived.Cards = []card.Card{}
		}

		m.updateAndResizeFocus()
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

		cardsToArchive := m.getSelectedOrFocusedCards()
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

		m.clearSelection()
		m.updateAndResizeFocus()
		m.clampFocusedCard()
	case "set":
		m.history.Drop() // Don't save state changes in history
		switch args {
		case "done":
			if len(m.displayColumns) > 0 {
				m.doneColumnName = m.displayColumns[m.focusedColumn].Title
				m.statusMessage = "Set done column to: " + m.doneColumnName
				return clearStatusCmd(3 * time.Second)
			}
		case "done?":
			if m.doneColumnName != "" {
				m.statusMessage = "Done column is: " + m.doneColumnName
			} else {
				m.statusMessage = "Done column is not set. Use `:set done`."
			}
			return clearStatusCmd(3 * time.Second)
		}
	case "unset":
		m.history.Drop() // Don't save state changes in history
		if args == "done" {
			if m.doneColumnName != "" {
				m.doneColumnName = ""
				m.statusMessage = "Done column has been unset."
			} else {
				m.statusMessage = "Done column was not set."
			}
			return clearStatusCmd(3 * time.Second)
		}
	case "done":
		if m.doneColumnName == "" {
			return nil
		}

		var destCol *column.Column
		for i := range m.board.Columns {
			if m.board.Columns[i].Title == m.doneColumnName {
				destCol = &m.board.Columns[i]
				break
			}
		}
		if destCol == nil {
			return nil
		}

		cardsToMove := m.getSelectedOrFocusedCards()
		if len(cardsToMove) == 0 {
			return nil
		}

		m.moveCards(cardsToMove, destCol)
		fs.WriteBoard(m.board)
		m.clearSelection()
		m.clampFocusedCard()
	case "show":
		m.history.Drop() // Don't save view changes in history
		if args == "hidden" {
			m.showHidden = true
			m.updateAndResizeFocus()
		}
	case "hide":
		m.history.Drop() // Don't save view changes in history
		if args == "hidden" {
			if m.focusedColumn < len(m.displayColumns) {
				focusedColTitle := m.displayColumns[m.focusedColumn].Title
				if focusedColTitle == fs.ArchiveColumnName {
					m.focusedColumn = 0
				}
			}
			m.showHidden = false
			m.updateAndResizeFocus()
		}
	case "right":
		if m.focusedColumn >= len(m.board.Columns)-1 {
			return nil
		}
		// Cannot move archive column
		if m.displayColumns[m.focusedColumn].Title == fs.ArchiveColumnName {
			return nil
		}

		i := m.focusedColumn
		m.board.Columns[i], m.board.Columns[i+1] = m.board.Columns[i+1], m.board.Columns[i]
		m.focusedColumn++
		fs.WriteBoard(m.board)
		m.updateAndResizeFocus()
		m.statusMessage = "Moved column right"
		return clearStatusCmd(2 * time.Second)
	case "left":
		if m.focusedColumn <= 0 {
			return nil
		}
		// Cannot move archive column
		if m.displayColumns[m.focusedColumn].Title == fs.ArchiveColumnName {
			return nil
		}

		i := m.focusedColumn
		m.board.Columns[i], m.board.Columns[i-1] = m.board.Columns[i-1], m.board.Columns[i]
		m.focusedColumn--
		fs.WriteBoard(m.board)
		m.updateAndResizeFocus()
		m.statusMessage = "Moved column left"
		return clearStatusCmd(2 * time.Second)
	case "noh", "nohlsearch":
		m.history.Drop() // Not an action to be undone
		m.lastSearchQuery = ""
		m.searchResults = []searchResult{}
		m.currentSearchResultIdx = -1
		m.statusMessage = "Search highlighting cleared"
		return clearStatusCmd(2 * time.Second)
	default:
		m.history.Drop() // Invalid command, don't save state
	}

	return nil
}
