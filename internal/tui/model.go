// internal/tui/model.go
package tui

import (
	"fmt"
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
	if m.width == 0 || len(m.board.Columns) == 0 {
		return 0
	}
	numColumns := len(m.board.Columns)
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
	if m.width == 0 || len(m.board.Columns) == 0 || m.focusedColumn >= len(m.board.Columns) {
		return 1
	}
	col := m.board.Columns[m.focusedColumn]
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
	if m.height == 0 || len(m.board.Columns) == 0 || m.focusedColumn >= len(m.board.Columns) {
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
	cards := m.board.Columns[m.focusedColumn].Cards

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
