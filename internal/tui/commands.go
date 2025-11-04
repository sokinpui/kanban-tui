// internal/tui/commands.go
package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/models"
)

type commandInfo struct {
	execute        func(m *Model, command, args string) tea.Cmd
	getCompletions func(args string) []string
}

var commandRegistry = make(map[string]commandInfo)

func registerCommand(name string, info commandInfo) {
	commandRegistry[name] = info
}

func init() {
	registerCommand("q", commandInfo{execute: cmdQuit})
	registerCommand("Q", commandInfo{execute: cmdQuit})
	registerCommand("wq", commandInfo{execute: cmdQuit})
	registerCommand("wQ", commandInfo{execute: cmdQuit})
	registerCommand("Wq", commandInfo{execute: cmdQuit})
	registerCommand("WQ", commandInfo{execute: cmdQuit})

	registerCommand("fzf", commandInfo{execute: cmdFzf})
	registerCommand("new", commandInfo{execute: cmdNew})
	registerCommand("sort", commandInfo{
		execute: cmdSort,
		getCompletions: func(args string) []string {
			return []string{"create", "modify", "name", "size"}
		},
	})
	registerCommand("create", commandInfo{execute: cmdCreateColumn})
	registerCommand("rename", commandInfo{execute: cmdRenameColumn})
	registerCommand("delete", commandInfo{execute: cmdDeleteColumn})
	registerCommand("archive", commandInfo{execute: cmdArchive})
	registerCommand("set", commandInfo{
		execute: cmdSet,
		getCompletions: func(args string) []string {
			return []string{"done", "done?"}
		},
	})
	registerCommand("unset", commandInfo{
		execute: cmdUnset,
		getCompletions: func(args string) []string {
			return []string{"done"}
		},
	})
	registerCommand("done", commandInfo{execute: cmdDone})
	registerCommand("show", commandInfo{
		execute: cmdShow,
		getCompletions: func(args string) []string {
			return []string{"hidden"}
		},
	})
	registerCommand("hide", commandInfo{
		execute: cmdHide,
		getCompletions: func(args string) []string {
			return []string{"hidden"}
		},
	})
	registerCommand("right", commandInfo{execute: cmdMoveColumnRight})
	registerCommand("left", commandInfo{execute: cmdMoveColumnLeft})
	registerCommand("noh", commandInfo{execute: cmdNoHighlight})
	registerCommand("nohlsearch", commandInfo{execute: cmdNoHighlight})
}

func cmdQuit(m *Model, command, args string) tea.Cmd {
	m.history.Drop()
	return tea.Quit
}

func cmdFzf(m *Model, command, args string) tea.Cmd {
	return m.openFZF()
}

func cmdNew(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	title := args
	currentCol := m.displayColumns[m.focusedColumn]

	newCard, err := m.client.CreateCard(*currentCol, title)
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

	currentCol.Cards = append(currentCol.Cards[:insertIndex], append([]models.Card{newCard}, currentCol.Cards[insertIndex:]...)...)

	if err := m.client.WriteBoard(m.board); err != nil {
		return nil
	}
	m.setCurrentFocusedCard(insertIndex + 1)
	m.ensureFocusedCardIsVisible()
	return nil
}

func cmdSort(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	descending := strings.HasSuffix(command, "!")
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
		m.history.Drop()
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

	m.client.WriteBoard(m.board)
	m.setCurrentFocusedCard(0)
	m.ensureFocusedCardIsVisible()
	return nil
}

func cmdCreateColumn(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	name := args
	if name == "" {
		m.history.Drop()
		return nil
	}
	newCol, err := m.client.CreateColumn(name)
	if err != nil {
		m.history.Drop()
		m.statusMessage = fmt.Sprintf("Error creating column: %v", err)
		return clearStatusCmd(3 * time.Second)
	}
	m.board.Columns = append(m.board.Columns, newCol)
	m.updateAndResizeFocus()
	m.client.WriteBoard(m.board)
	m.focusedColumn = len(m.displayColumns) - 1
	return nil
}

func cmdRenameColumn(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
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

	if oldName == "Archived" { // fs.ArchiveColumnName
		m.history.Drop()
		m.statusMessage = "Cannot rename the Archived column"
		return clearStatusCmd(3 * time.Second)
	}

	if err := m.client.RenameColumn(colToRename, newName); err != nil {
		m.history.Drop()
		m.statusMessage = fmt.Sprintf("Error renaming column: %v", err)
		return clearStatusCmd(5 * time.Second)
	}

	if m.doneColumnName == oldName {
		m.doneColumnName = newName
	}

	m.client.WriteBoard(m.board)
	m.statusMessage = fmt.Sprintf("Renamed column '%s' to '%s'", oldName, newName)
	return clearStatusCmd(3 * time.Second)
}

func cmdDeleteColumn(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	if m.currentFocusedCard() != 0 || len(m.displayColumns) == 0 {
		m.history.Drop()
		return nil
	}

	colToDelete := m.board.Columns[m.focusedColumn]
	if err := m.client.DeleteColumn(colToDelete); err != nil {
		m.history.Drop()
		m.statusMessage = fmt.Sprintf("Error deleting column: %v", err)
		return clearStatusCmd(3 * time.Second)
	}

	if colToDelete.Title != "Archived" { // fs.ArchiveColumnName
		var newCols []models.Column
		for _, c := range m.board.Columns {
			if c.Title != colToDelete.Title {
				newCols = append(newCols, c)
			}
		}
		m.board.Columns = newCols
	} else {
		m.board.Archived.Cards = []models.Card{}
	}

	m.updateAndResizeFocus()
	m.client.WriteBoard(m.board)
	return nil
}

func cmdArchive(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	if len(m.selected) == 0 {
		m.history.Drop()
		return nil
	}

	if _, err := os.Stat(m.board.Archived.Path); os.IsNotExist(err) {
		if err := os.MkdirAll(m.board.Archived.Path, 0755); err != nil {
			m.history.Drop()
			m.statusMessage = fmt.Sprintf("Error creating archive dir: %v", err)
			return clearStatusCmd(3 * time.Second)
		}
	}

	cardsToArchive := m.getSelectedOrFocusedCards()
	movedCards := make([]models.Card, 0, len(cardsToArchive))
	for _, c := range cardsToArchive {
		err := m.client.MoveCard(c, m.board.Archived)
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

	m.client.WriteBoard(m.board)

	m.clearSelection()
	m.updateAndResizeFocus()
	m.clampFocusedCard()
	return nil
}

func cmdSet(m *Model, command, args string) tea.Cmd {
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
	return nil
}

func cmdUnset(m *Model, command, args string) tea.Cmd {
	if args == "done" {
		if m.doneColumnName != "" {
			m.doneColumnName = ""
			m.statusMessage = "Done column has been unset."
		} else {
			m.statusMessage = "Done column was not set."
		}
		return clearStatusCmd(3 * time.Second)
	}
	return nil
}

func cmdDone(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	if m.doneColumnName == "" {
		m.history.Drop()
		return nil
	}

	var destCol *models.Column
	for i := range m.board.Columns {
		if m.board.Columns[i].Title == m.doneColumnName {
			destCol = &m.board.Columns[i]
			break
		}
	}
	if destCol == nil {
		m.history.Drop()
		return nil
	}

	cardsToMove := m.getSelectedOrFocusedCards()
	if len(cardsToMove) == 0 {
		m.history.Drop()
		return nil
	}

	m.moveCards(cardsToMove, destCol)
	m.client.WriteBoard(m.board)
	m.clearSelection()
	m.clampFocusedCard()
	return nil
}

func cmdShow(m *Model, command, args string) tea.Cmd {
	if args == "hidden" {
		m.showHidden = true
		m.updateAndResizeFocus()
	}
	return nil
}

func cmdHide(m *Model, command, args string) tea.Cmd {
	if args == "hidden" {
		if m.focusedColumn < len(m.displayColumns) {
			focusedColTitle := m.displayColumns[m.focusedColumn].Title
			if focusedColTitle == "Archived" { // fs.ArchiveColumnName
				m.focusedColumn = 0
			}
		}
		m.showHidden = false
		m.updateAndResizeFocus()
	}
	return nil
}

func cmdMoveColumnRight(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	if m.focusedColumn >= len(m.board.Columns)-1 {
		m.history.Drop()
		return nil
	}
	if m.displayColumns[m.focusedColumn].Title == "Archived" { // fs.ArchiveColumnName
		m.history.Drop()
		return nil
	}

	i := m.focusedColumn
	m.board.Columns[i], m.board.Columns[i+1] = m.board.Columns[i+1], m.board.Columns[i]
	m.focusedColumn++
	m.client.WriteBoard(m.board)
	m.updateAndResizeFocus()
	m.statusMessage = "Moved column right"
	return clearStatusCmd(2 * time.Second)
}

func cmdMoveColumnLeft(m *Model, command, args string) tea.Cmd {
	m.saveStateForUndo()
	if m.focusedColumn <= 0 {
		m.history.Drop()
		return nil
	}
	if m.displayColumns[m.focusedColumn].Title == "Archived" { // fs.ArchiveColumnName
		m.history.Drop()
		return nil
	}

	i := m.focusedColumn
	m.board.Columns[i], m.board.Columns[i-1] = m.board.Columns[i-1], m.board.Columns[i]
	m.focusedColumn--
	m.client.WriteBoard(m.board)
	m.updateAndResizeFocus()
	m.statusMessage = "Moved column left"
	return clearStatusCmd(2 * time.Second)
}

func cmdNoHighlight(m *Model, command, args string) tea.Cmd {
	m.lastSearchQuery = ""
	m.searchResults = []searchResult{}
	m.currentSearchResultIdx = -1
	m.statusMessage = "Search highlighting cleared"
	return clearStatusCmd(2 * time.Second)
}
