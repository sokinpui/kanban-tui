// internal/tui/model.go
package tui

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kanban/internal/board"
	"kanban/internal/card"
	"kanban/internal/fs"
)

type mode int

const (
	normalMode mode = iota
	commandMode
	visualMode
)

type editorFinishedMsg struct {
	err  error
	path string
}

func openEditor(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err, path: path}
	})
}

type Model struct {
	board             board.Board
	focusedColumn     int
	columnCardFocus   []int
	mode              mode
	textInput         textinput.Model
	width             int
	height            int
	selected          map[string]struct{}
	clipboard         []card.Card
	isCut             bool
	scrollOffset      int
	createCardMode    string
	lastGPress        time.Time
	lastYPress        time.Time
	lastDPress        time.Time
	visualSelectStart int
}

func NewModel(b board.Board, focusedColumn, focusedCard int) Model {
	ti := textinput.New()
	ti.Prompt = ":"

	m := Model{
		board:             b,
		columnCardFocus:   make([]int, len(b.Columns)),
		mode:              normalMode,
		textInput:         ti,
		selected:          make(map[string]struct{}),
		clipboard:         []card.Card{},
		scrollOffset:      0,
		createCardMode:    "prepend",
		visualSelectStart: -1,
	}

	if len(m.board.Columns) == 0 {
		m.focusedColumn = 0
	} else {
		if focusedColumn < 0 {
			focusedColumn = 0
		}
		if focusedColumn >= len(m.board.Columns) {
			focusedColumn = len(m.board.Columns) - 1
		}
		m.focusedColumn = focusedColumn
		m.columnCardFocus[m.focusedColumn] = focusedCard
		m.clampFocusedCard()
	}

	return m
}

func (m Model) FocusedColumn() int {
	return m.focusedColumn
}

func (m Model) FocusedCard() int {
	if m.focusedColumn < 0 || m.focusedColumn >= len(m.columnCardFocus) {
		return 0
	}
	return m.columnCardFocus[m.focusedColumn]
}

func (m *Model) currentFocusedCard() int {
	if m.focusedColumn < 0 || m.focusedColumn >= len(m.columnCardFocus) {
		return 0
	}
	return m.columnCardFocus[m.focusedColumn]
}

func (m *Model) setCurrentFocusedCard(focus int) {
	if m.focusedColumn >= 0 && m.focusedColumn < len(m.columnCardFocus) {
		m.columnCardFocus[m.focusedColumn] = focus
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textInput.Width = m.width
		m.ensureFocusedCardIsVisible()
		return m, nil

	case editorFinishedMsg:
		if msg.err != nil {
			return m, nil
		}
		currentFocus := m.currentFocusedCard()
		if currentFocus > 0 {
			updatedCard, err := fs.LoadCard(msg.path)
			if err != nil {
				return m, nil
			}
			m.board.Columns[m.focusedColumn].Cards[currentFocus-1] = updatedCard
			fs.WriteBoard(m.board)
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.mode {
	case commandMode:
		cmd = m.updateCommandMode(msg)
	case visualMode:
		cmd = m.updateVisualMode(msg)
	default: // normalMode
		cmd = m.updateNormalMode(msg)
	}
	return m, cmd
}

func (m Model) View() string {
	if m.height == 0 || m.width == 0 {
		return ""
	}

	boardView := renderBoard(&m)
	statusBar := renderStatusBar(&m)

	statusBarHeight := lipgloss.Height(statusBar)
	boardHeight := m.height - statusBarHeight

	boardContainer := lipgloss.NewStyle().
		Height(boardHeight).
		MaxHeight(boardHeight).
		Render(boardView)

	return lipgloss.JoinVertical(lipgloss.Left, boardContainer, statusBar)
}

func (m *Model) updateNormalMode(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "q", "ctrl+c":
		return tea.Quit

	case "esc":
		m.selected = make(map[string]struct{})
		m.clipboard = []card.Card{}
		m.isCut = false

	case ":":
		m.mode = commandMode
		m.textInput.SetValue("")
		return m.textInput.Focus()

	case "h", "left":
		if m.focusedColumn > 0 {
			m.focusedColumn--
			m.clampFocusedCard()
			m.ensureFocusedCardIsVisible()
		}

	case "l", "right":
		if m.focusedColumn < len(m.board.Columns)-1 {
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
		if currentFocus < len(m.board.Columns[m.focusedColumn].Cards) {
			m.setCurrentFocusedCard(currentFocus + 1)
			m.ensureFocusedCardIsVisible()
		}

	case "g":
		if time.Since(m.lastGPress) < 500*time.Millisecond {
			if len(m.board.Columns[m.focusedColumn].Cards) > 0 {
				m.setCurrentFocusedCard(1)
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
			m.ensureFocusedCardIsVisible()
		}

	case "enter":
		currentFocus := m.currentFocusedCard()
		if currentFocus > 0 {
			cardToEdit := m.board.Columns[m.focusedColumn].Cards[currentFocus-1]
			return openEditor(cardToEdit.Path)
		}

	case "O":
		m.createCardMode = "before"
		m.mode = commandMode
		m.textInput.SetValue("new ")
		return m.textInput.Focus()

	case "o":
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
				m.clipboard = []card.Card{m.board.Columns[m.focusedColumn].Cards[currentFocus-1]}
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
				m.clipboard = []card.Card{m.board.Columns[m.focusedColumn].Cards[currentFocus-1]}
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

		sort.Slice(m.clipboard, func(i, j int) bool {
			return m.clipboard[i].CreatedAt.After(m.clipboard[j].CreatedAt)
		})

		destCol := &m.board.Columns[m.focusedColumn]

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

		var cardsToInsert []card.Card
		if m.isCut {
			clipboardUUIDs := make(map[string]struct{}, len(m.clipboard))
			for i := range m.clipboard {
				c := &m.clipboard[i]
				clipboardUUIDs[c.UUID] = struct{}{}
				fs.MoveCard(c, *destCol)
			}

			for i := range m.board.Columns {
				col := &m.board.Columns[i]
				keptCards := col.Cards[:0]
				for _, c := range col.Cards {
					if _, found := clipboardUUIDs[c.UUID]; !found {
						keptCards = append(keptCards, c)
					}
				}
				col.Cards = keptCards
			}
			cardsToInsert = m.clipboard
		} else {
			var newCards []card.Card
			for _, c := range m.clipboard {
				newCard, err := fs.CopyCard(c, *destCol)
				if err == nil {
					newCards = append(newCards, newCard)
				}
			}
			cardsToInsert = newCards
		}
		if len(cardsToInsert) > 0 {
			destCol.Cards = append(destCol.Cards[:insertIndex], append(cardsToInsert, destCol.Cards[insertIndex:]...)...)
			fs.WriteBoard(m.board)
			m.clipboard = []card.Card{}
			m.isCut = false
		}

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
			cardsToDelete = append(cardsToDelete, m.board.Columns[m.focusedColumn].Cards[cardIndex])
		}

		if len(cardsToDelete) == 0 {
			return nil
		}

		trashedUUIDs := make(map[string]struct{})
		for _, c := range cardsToDelete {
			if err := fs.TrashCard(c); err == nil {
				trashedUUIDs[c.UUID] = struct{}{}
			}
		}

		if len(trashedUUIDs) > 0 {
			for i := range m.board.Columns {
				col := &m.board.Columns[i]
				keptCards := col.Cards[:0]
				for _, c := range col.Cards {
					if _, wasTrashed := trashedUUIDs[c.UUID]; !wasTrashed {
						keptCards = append(keptCards, c)
					}
				}
				col.Cards = keptCards
			}

			keptClipboard := m.clipboard[:0]
			for _, c := range m.clipboard {
				if _, wasTrashed := trashedUUIDs[c.UUID]; !wasTrashed {
					keptClipboard = append(keptClipboard, c)
				}
			}
			m.clipboard = keptClipboard

			m.selected = make(map[string]struct{})
			fs.WriteBoard(m.board)
			m.clampFocusedCard()
		}
	}
	return nil
}

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
		if title == "" {
			return nil
		}
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
				insertIndex = len(currentCol.Cards)
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
	}

	return nil
}

func (m *Model) clampFocusedCard() {
	if len(m.board.Columns) == 0 {
		return
	}
	maxIndex := m.board.Columns[m.focusedColumn].CardCount()

	currentFocus := m.currentFocusedCard()
	if currentFocus < 0 {
		currentFocus = 0
	}
	if currentFocus > maxIndex {
		currentFocus = maxIndex
	}
	m.setCurrentFocusedCard(currentFocus)
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

func (m Model) isCardMarkedForCut(uuid string) bool {
	if !m.isCut {
		return false
	}
	for _, c := range m.clipboard {
		if c.UUID == uuid {
			return true
		}
	}
	return false
}

func (m *Model) cardAreaHeight() int {
	statusBarHeight := 0
	if m.mode == commandMode {
		statusBarHeight = 1
	}
	headerHeight := 1
	return m.height - statusBarHeight - headerHeight
}

func (m *Model) cardWidth(columnWidth int) int {
	return columnWidth - (columnPaddingHorizontal * 2) - (cardMarginHorizontal * 2)
}

func (m *Model) cardContentWidth(columnWidth int) int {
	cardW := m.cardWidth(columnWidth)
	cardPadding := 2 // cardStyle.Padding(0, 1) -> 1 left, 1 right
	cardBorder := 2  // 1 left, 1 right
	return cardW - cardPadding - cardBorder
}

func (m *Model) getFocusedColumnWidth() int {
	if m.width == 0 || len(m.board.Columns) == 0 {
		return 0
	}
	numColumns := len(m.board.Columns)
	baseColumnWidth := m.width / numColumns
	remainder := m.width % numColumns

	colWidth := baseColumnWidth
	if m.focusedColumn < remainder {
		colWidth++
	}
	return colWidth
}

func (m *Model) getCardRenderHeight(c card.Card) int {
	focusedColWidth := m.getFocusedColumnWidth()
	if focusedColWidth == 0 {
		return 2
	}
	contentW := m.cardContentWidth(focusedColWidth)
	if contentW < 1 {
		return 2 // just border height
	}
	contentStyle := lipgloss.NewStyle().Width(contentW)
	contentHeight := lipgloss.Height(contentStyle.Render(c.Title))
	borderHeight := 2 // For lipgloss.RoundedBorder
	return contentHeight + borderHeight
}

func (m *Model) ensureFocusedCardIsVisible() {
	if m.height == 0 {
		return
	}

	currentFocus := m.currentFocusedCard()
	if currentFocus == 0 {
		m.scrollOffset = 0
		return
	}

	focusedIdx := currentFocus - 1

	if focusedIdx < m.scrollOffset {
		m.scrollOffset = focusedIdx
		return
	}

	cardAreaH := m.cardAreaHeight()
	cards := m.board.Columns[m.focusedColumn].Cards

	currentHeight := 0
	lastVisibleIdx := -1

	for i := m.scrollOffset; i < len(cards); i++ {
		cardHeight := m.getCardRenderHeight(cards[i])
		separatorHeight := 0
		if i > m.scrollOffset {
			separatorHeight = 1
		}

		if currentHeight+cardHeight+separatorHeight > cardAreaH {
			break
		}

		currentHeight += cardHeight + separatorHeight
		lastVisibleIdx = i
	}

	if focusedIdx > lastVisibleIdx {
		newOffset := focusedIdx
		visibleHeight := 0
		for {
			cardHeight := m.getCardRenderHeight(cards[newOffset])
			separatorHeight := 0
			if newOffset < focusedIdx {
				separatorHeight = 1
			}

			if visibleHeight+cardHeight+separatorHeight > cardAreaH {
				newOffset++
				break
			}
			visibleHeight += cardHeight + separatorHeight

			if newOffset == 0 {
				break
			}
			newOffset--
		}
		m.scrollOffset = newOffset
	}
}
