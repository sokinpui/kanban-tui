// internal/tui/model.go
package tui

import (
	"fmt"
	"strings"
	"time"
	"os"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kanban/internal/board"
	"kanban/internal/card"
	"kanban/internal/column"
	"kanban/internal/history"
	"kanban/internal/fs"
)

type clearStatusMsg struct{}

func clearStatusCmd(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

type mode int

const (
	normalMode mode = iota
	commandMode
	visualMode
	searchMode
	fzfMode
)

type searchResult struct {
	colIndex  int
	cardIndex int // 1-based, like focus
}

type boardSession struct {
	board           board.Board
	focusedColumn   int
	columnCardFocus []int
	scrollOffset    int
	doneColumnName  string
	showHidden      bool
}

type Model struct {
	board             board.Board
	// boardStack allows navigating into linked boards and returning.
	boardStack        []boardSession

	displayColumns    []*column.Column
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
	doneColumnName    string
	showHidden        bool
	history           *history.History
	lastCommand       string
	statusMessage     string
	fzf               FZFModel

	completionMatches      []string
	completionIndex        int
	lastSearchQuery        string
	lastSearchDirection    string // "/" or "?"
	searchResults          []searchResult
	currentSearchResultIdx int
}

func NewModel(b board.Board, state *fs.AppState) Model {
	ti := textinput.New()
	ti.Prompt = ":"

	m := Model{
		board:             b,
		boardStack:        []boardSession{},
		mode:              normalMode,
		textInput:         ti,
		selected:          make(map[string]struct{}),
		clipboard:         []card.Card{},
		scrollOffset:      0,
		createCardMode:    "prepend",
		visualSelectStart: -1,
		doneColumnName:    state.DoneColumn,
		showHidden:        state.ShowHidden,
		history:           history.New(),
		searchResults:     []searchResult{},
		fzf:               NewFZFModel(),
		completionMatches:      []string{},
		completionIndex:        -1,
		currentSearchResultIdx: -1,
	}
	m.updateDisplayColumns()

	m.columnCardFocus = make([]int, len(m.displayColumns))

	if len(m.displayColumns) == 0 {
		m.focusedColumn = 0
	} else {
		focusedColumn := state.FocusedColumn
		if focusedColumn < 0 {
			focusedColumn = 0
		}
		if focusedColumn >= len(m.displayColumns) {
			focusedColumn = len(m.displayColumns) - 1
		}
		m.focusedColumn = focusedColumn
		m.columnCardFocus[m.focusedColumn] = state.FocusedCard
		m.clampFocusedCard()
	}

	return m
}

func (m *Model) updateDisplayColumns() {
	m.displayColumns = make([]*column.Column, len(m.board.Columns))
	for i := range m.board.Columns {
		m.displayColumns[i] = &m.board.Columns[i]
	}

	if m.showHidden && m.board.Archived.CardCount() > 0 {
		m.displayColumns = append(m.displayColumns, &m.board.Archived)
	}
}

func (m Model) State() fs.AppState {
	return fs.AppState{
		FocusedColumn: m.FocusedColumn(),
		FocusedCard:   m.FocusedCard(),
		DoneColumn:    m.doneColumnName,
		ShowHidden:    m.showHidden,
	}
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
		m.fzf.SetSize(m.width, m.height)
		m.textInput.Width = m.width
		m.ensureFocusedCardIsVisible()
		return m, nil

	case fzfCardSelectedMsg:
		m.mode = normalMode
		m.fzf.Blur()
		found := false
		for colIdx, col := range m.displayColumns {
			for cardIdx, c := range col.Cards {
				if c.UUID == msg.card.UUID {
					m.focusedColumn = colIdx
					m.setCurrentFocusedCard(cardIdx + 1)
					m.ensureFocusedCardIsVisible()
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		return m, nil

	case fzfCancelledMsg:
		m.mode = normalMode
		m.fzf.Blur()
		return m, nil

	case clearStatusMsg:
		m.statusMessage = ""
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
			m.displayColumns[m.focusedColumn].Cards[currentFocus-1] = updatedCard
			fs.WriteBoard(m.board)
		}
		return m, nil

	case boardSwitchedMsg:
		if msg.err != nil {
			m.statusMessage = msg.err.Error()
			return m, clearStatusCmd(4 * time.Second)
		}

		// Why: Save the state of the current board before navigating away.
		currentState := m.State()
		if err := fs.SaveState(currentState.FocusedColumn, currentState.FocusedCard, currentState.DoneColumn, currentState.ShowHidden); err != nil {
			m.statusMessage = fmt.Sprintf("Error saving state: %v", err)
			return m, clearStatusCmd(4 * time.Second)
		}

		session := boardSession{
			board:           m.board,
			focusedColumn:   m.focusedColumn,
			columnCardFocus: m.columnCardFocus,
			scrollOffset:    m.scrollOffset,
			doneColumnName:  m.doneColumnName,
			showHidden:      m.showHidden,
		}
		m.boardStack = append(m.boardStack, session)

		if err := os.Chdir(msg.path); err != nil {
			// If we can't change directory, we can't proceed. Roll back the stack push.
			m.boardStack = m.boardStack[:len(m.boardStack)-1]
			m.statusMessage = fmt.Sprintf("Error changing directory: %v", err)
			return m, clearStatusCmd(4 * time.Second)
		}

		m.reInit(msg.board, &msg.state)
		m.statusMessage = "Switched to board: " + msg.board.Path
		return m, clearStatusCmd(2 * time.Second)
	}

	var cmd tea.Cmd
	switch m.mode {
	case fzfMode:
		newFzf, cmd := m.fzf.Update(msg)
		m.fzf = newFzf.(FZFModel)
		return m, cmd
	case commandMode:
		cmd = m.updateCommandMode(msg)
	case visualMode:
		cmd = m.updateVisualMode(msg)
	case searchMode:
		cmd = m.updateSearchMode(msg)
	default: // normalMode
		cmd = m.updateNormalMode(msg)
	}
	return m, cmd
}

func (m *Model) reInit(b board.Board, state *fs.AppState) {
	ti := textinput.New()
	ti.Prompt = ":"

	// Preserve window size
	width, height := m.width, m.height
	boardStack := m.boardStack

	// Re-initialize the model struct
	*m = Model{
		width:             width,
		height:            height,
		boardStack:        boardStack,
		board:             b,
		mode:              normalMode,
		textInput:         ti,
		selected:          make(map[string]struct{}),
		clipboard:         []card.Card{},
		scrollOffset:      0,
		createCardMode:    "prepend",
		visualSelectStart: -1,
		doneColumnName:    state.DoneColumn,
		showHidden:        state.ShowHidden,
		history:           history.New(),
		searchResults:     []searchResult{},
		fzf:               NewFZFModel(),
		completionMatches:      []string{},
		completionIndex:        -1,
		currentSearchResultIdx: -1,
	}

	m.fzf.SetSize(m.width, m.height)
	m.updateDisplayColumns()

	m.columnCardFocus = make([]int, len(m.displayColumns))

	if len(m.displayColumns) == 0 {
		m.focusedColumn = 0
	} else {
		focusedColumn := state.FocusedColumn
		if focusedColumn < 0 {
			focusedColumn = 0
		}
		if focusedColumn >= len(m.displayColumns) {
			focusedColumn = len(m.displayColumns) - 1
		}
		m.focusedColumn = focusedColumn
		m.columnCardFocus[m.focusedColumn] = state.FocusedCard
		m.clampFocusedCard()
	}
}

func (m Model) View() string {
	if m.height == 0 || m.width == 0 {
		return ""
	}

	if m.mode == fzfMode {
		return m.fzf.View()
	}

	statusBar := renderStatusBar(&m)
	statusBarHeight := lipgloss.Height(statusBar)
	boardHeight := m.height - statusBarHeight

	boardView := renderBoard(&m, boardHeight)

	if statusBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, boardView, statusBar)
	}
	return boardView
}

func (m *Model) deleteCards(cardsToDelete []card.Card) {
	if len(cardsToDelete) == 0 {
		return
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

func (m *Model) clampFocusedCard() {
	if len(m.displayColumns) == 0 {
		return
	}
	maxIndex := m.displayColumns[m.focusedColumn].CardCount()

	currentFocus := m.currentFocusedCard()
	if currentFocus < 0 {
		currentFocus = 0
	}
	if currentFocus > maxIndex {
		currentFocus = maxIndex
	}
	m.setCurrentFocusedCard(currentFocus)
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
	if m.width == 0 || len(m.displayColumns) == 0 {
		return 0
	}
	numColumns := len(m.displayColumns)
	numSeparators := numColumns - 1
	if numSeparators < 0 {
		numSeparators = 0
	}

	availableWidth := m.width - numSeparators
	baseColumnWidth := availableWidth / numColumns
	remainder := availableWidth % numColumns

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

func (m *Model) getColumnHeaderHeight() int {
	if m.width == 0 || len(m.displayColumns) == 0 || m.focusedColumn >= len(m.displayColumns) {
		return 1
	}
	col := m.displayColumns[m.focusedColumn]
	isHeaderFocused := m.currentFocusedCard() == 0

	header := fmt.Sprintf("%s %d", col.Title, col.CardCount())

	headerStyle := columnHeaderStyle
	if isHeaderFocused {
		headerStyle = focusedColumnHeaderStyle
	}

	colWidth := m.getFocusedColumnWidth()
	if colWidth == 0 {
		return 1
	}

	headerContentWidth := colWidth - columnStyle.GetHorizontalPadding() - headerStyle.GetHorizontalPadding()
	if headerContentWidth < 0 {
		headerContentWidth = 0
	}
	renderedHeader := headerStyle.Copy().Width(headerContentWidth).Render(header)
	return lipgloss.Height(renderedHeader)
}

func (m *Model) ensureFocusedCardIsVisible() {
	if m.height == 0 || len(m.displayColumns) == 0 || m.focusedColumn >= len(m.displayColumns) {
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

	statusBar := renderStatusBar(m)
	statusBarHeight := lipgloss.Height(statusBar)
	headerHeight := m.getColumnHeaderHeight()
	cardAreaH := m.height - statusBarHeight - headerHeight
	if cardAreaH < 0 {
		cardAreaH = 0
	}
	cards := m.displayColumns[m.focusedColumn].Cards

	currentHeight := 0
	lastVisibleIdx := -1
	visibleCardsCount := 0

	for i := m.scrollOffset; i < len(cards); i++ {
		cardHeight := m.getCardRenderHeight(cards[i])

		heightToAdd := cardHeight
		if visibleCardsCount > 0 {
			heightToAdd++ // For the newline separator
		}

		if currentHeight+heightToAdd > cardAreaH {
			break
		}

		currentHeight += heightToAdd
		lastVisibleIdx = i
		visibleCardsCount++
	}

	if focusedIdx > lastVisibleIdx {
		newOffset := focusedIdx
		visibleHeight := 0
		visibleCardsCount := 0
		for {
			cardHeight := m.getCardRenderHeight(cards[newOffset])

			heightToAdd := cardHeight
			if visibleCardsCount > 0 {
				heightToAdd++ // For the newline separator
			}

			if visibleHeight+heightToAdd > cardAreaH {
				newOffset++
				break
			}
			visibleHeight += heightToAdd
			visibleCardsCount++

			if newOffset == 0 {
				break
			}
			newOffset--
		}
		m.scrollOffset = newOffset
	}
}

func (m *Model) getSelectedOrFocusedCards() []*card.Card {
	cardsToMove := make([]*card.Card, 0)
	if len(m.selected) > 0 {
		for i := range m.board.Columns {
			for j := range m.board.Columns[i].Cards {
				c := &m.board.Columns[i].Cards[j]
				if _, isSelected := m.selected[c.UUID]; isSelected {
					cardsToMove = append(cardsToMove, c)
				}
			}
		}
		if m.showHidden {
			for j := range m.board.Archived.Cards {
				c := &m.board.Archived.Cards[j]
				if _, isSelected := m.selected[c.UUID]; isSelected {
					cardsToMove = append(cardsToMove, c)
				}
			}
		}
	} else if m.currentFocusedCard() > 0 {
		cardIndex := m.currentFocusedCard() - 1
		focusedCol := m.displayColumns[m.focusedColumn]
		if cardIndex < len(focusedCol.Cards) {
			cardsToMove = append(cardsToMove, &focusedCol.Cards[cardIndex])
		}
	}
	return cardsToMove
}

func (m *Model) moveCards(cardsToMove []*card.Card, destCol *column.Column) {
	if len(cardsToMove) == 0 {
		return
	}

	successfullyMovedCards := make([]card.Card, 0, len(cardsToMove))
	movedUUIDs := make(map[string]struct{})
	for _, c := range cardsToMove {
		err := fs.MoveCard(c, *destCol)
		if err == nil {
			successfullyMovedCards = append(successfullyMovedCards, *c)
			movedUUIDs[c.UUID] = struct{}{}
		}
	}

	if len(successfullyMovedCards) == 0 {
		return
	}

	// Remove from ALL source columns by creating new slices.
	for i := range m.board.Columns {
		col := &m.board.Columns[i]
		keptCards := make([]card.Card, 0, len(col.Cards))
		for _, c := range col.Cards {
			if _, wasMoved := movedUUIDs[c.UUID]; !wasMoved {
				keptCards = append(keptCards, c)
			}
		}
		col.Cards = keptCards
	}

	keptArchived := make([]card.Card, 0, len(m.board.Archived.Cards))
	for _, c := range m.board.Archived.Cards {
		if _, wasMoved := movedUUIDs[c.UUID]; !wasMoved {
			keptArchived = append(keptArchived, c)
		}
	}
	m.board.Archived.Cards = keptArchived

	destCol.Cards = append(destCol.Cards, successfullyMovedCards...)
}

func (m *Model) clearSelection() {
	m.selected = make(map[string]struct{})
	m.clipboard = []card.Card{}
	m.isCut = false
	m.visualSelectStart = -1
	m.mode = normalMode
}

func (m *Model) saveStateForUndo() {
	m.history.Push(m.board)
}

func (m *Model) updateAndResizeFocus() {
	m.updateDisplayColumns()

	newFocus := make([]int, len(m.displayColumns))
	copy(newFocus, m.columnCardFocus)
	m.columnCardFocus = newFocus

	if m.focusedColumn >= len(m.displayColumns) {
		m.focusedColumn = len(m.displayColumns) - 1
		if m.focusedColumn < 0 {
			m.focusedColumn = 0
		}
	}
	m.clampFocusedCard()
}

func (m *Model) jumpTo(res searchResult) {
	m.focusedColumn = res.colIndex
	m.setCurrentFocusedCard(res.cardIndex)
	m.ensureFocusedCardIsVisible()
}

func (m *Model) openFZF() tea.Cmd {
	m.mode = fzfMode

	m.lastSearchQuery = ""
	m.searchResults = []searchResult{}
	m.currentSearchResultIdx = -1

	var items []FzfItem
	for i := range m.board.Columns {
		col := m.board.Columns[i]
		for _, c := range col.Cards {
			items = append(items, FzfItem{Card: c, ColTitle: col.Title})
		}
	}
	if m.showHidden && m.board.Archived.CardCount() > 0 {
		for _, c := range m.board.Archived.Cards {
			items = append(items, FzfItem{Card: c, ColTitle: m.board.Archived.Title})
		}
	}
	m.fzf.SetItems(items)
	return m.fzf.Focus()
}

func (m *Model) popBoard() tea.Cmd {
	if len(m.boardStack) == 0 {
		return tea.Quit
	}

	// Why: Save the state of the board we are leaving.
	currentState := m.State()
	if err := fs.SaveState(currentState.FocusedColumn, currentState.FocusedCard, currentState.DoneColumn, currentState.ShowHidden); err != nil {
		m.statusMessage = fmt.Sprintf("Error saving state: %v", err)
		return clearStatusCmd(4 * time.Second)
	}

	lastSession := m.boardStack[len(m.boardStack)-1]
	m.boardStack = m.boardStack[:len(m.boardStack)-1]

	if err := os.Chdir(lastSession.board.Path); err != nil {
		// This is a fatal error, as the application state is now desynced
		// from the filesystem. It's safest to quit.
		fmt.Fprintf(os.Stderr, "FATAL: could not chdir back to %s: %v\n", lastSession.board.Path, err)
		return tea.Quit
	}

	m.board = lastSession.board
	m.focusedColumn = lastSession.focusedColumn
	m.columnCardFocus = lastSession.columnCardFocus
	m.scrollOffset = lastSession.scrollOffset
	m.doneColumnName = lastSession.doneColumnName
	m.showHidden = lastSession.showHidden

	m.updateDisplayColumns()
	m.clampFocusedCard()
	m.ensureFocusedCardIsVisible()

	m.statusMessage = "Returned to board: " + m.board.Path
	return clearStatusCmd(2 * time.Second)
}

func (m *Model) ExecuteCommand(commandStr string) tea.Cmd {
	parts := strings.SplitN(strings.TrimSpace(commandStr), " ", 2)
	command := parts[0]
	var args string
	if len(parts) > 1 {
		args = parts[1]
	}

	lookupName := command
	if strings.HasSuffix(lookupName, "!") {
		lookupName = strings.TrimSuffix(lookupName, "!")
	}

	cmdInfo, ok := commandRegistry[lookupName]
	if !ok {
		// This handles aliases like 'wq' which don't have a '!' variant
		cmdInfo, ok = commandRegistry[command]
	}

	if !ok {
		m.statusMessage = fmt.Sprintf("Not a command: %s", command)
		return clearStatusCmd(2 * time.Second)
	}

	// Why: The command itself is responsible for saving state for undo,
	// because not all commands are undoable (e.g., view changes).
	return cmdInfo.execute(m, command, args)
}
