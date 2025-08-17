// internal/tui/model.go
package tui

import (
	"os"
	"os/exec"
	"strings"

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
	board         board.Board
	focusedColumn int
	focusedCard   int
	mode          mode
	textInput     textinput.Model
	width         int
	height        int
	selected      map[string]struct{}
	clipboard     []card.Card
	isCut         bool
	scrollOffset  int
}

func NewModel(b board.Board, focusedColumn, focusedCard int) Model {
	ti := textinput.New()
	ti.Prompt = ":"

	m := Model{
		board:         b,
		focusedColumn: focusedColumn,
		focusedCard:   focusedCard,
		mode:          normalMode,
		textInput:     ti,
		selected:      make(map[string]struct{}),
		clipboard:     []card.Card{},
		scrollOffset:  0,
	}

	if len(m.board.Columns) == 0 {
		m.focusedColumn = 0
		m.focusedCard = 0
	} else {
		if m.focusedColumn < 0 {
			m.focusedColumn = 0
		}
		if m.focusedColumn >= len(m.board.Columns) {
			m.focusedColumn = len(m.board.Columns) - 1
		}
		m.clampFocusedCard()
	}

	return m
}

func (m Model) FocusedColumn() int {
	return m.focusedColumn
}

func (m Model) FocusedCard() int {
	return m.focusedCard
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
		if m.focusedCard > 0 {
			updatedCard, err := fs.LoadCard(msg.path)
			if err != nil {
				return m, nil
			}
			m.board.Columns[m.focusedColumn].Cards[m.focusedCard-1] = updatedCard
			fs.WriteBoard(m.board)
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.mode {
	case commandMode:
		cmd = m.updateCommandMode(msg)
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
		if m.focusedCard > 0 {
			m.focusedCard--
			m.ensureFocusedCardIsVisible()
		}

	case "j", "down":
		if m.focusedCard < len(m.board.Columns[m.focusedColumn].Cards) {
			m.focusedCard++
			m.ensureFocusedCardIsVisible()
		}

	case "enter":
		if m.focusedCard > 0 {
			cardToEdit := m.board.Columns[m.focusedColumn].Cards[m.focusedCard-1]
			return openEditor(cardToEdit.Path)
		}

	case " ":
		if m.focusedCard > 0 {
			c := m.board.Columns[m.focusedColumn].Cards[m.focusedCard-1]
			if _, ok := m.selected[c.UUID]; ok {
				delete(m.selected, c.UUID)
			} else {
				m.selected[c.UUID] = struct{}{}
			}
		}

	case "v":
		if len(m.board.Columns[m.focusedColumn].Cards) > 0 {
			for _, c := range m.board.Columns[m.focusedColumn].Cards {
				if _, ok := m.selected[c.UUID]; ok {
					delete(m.selected, c.UUID)
				} else {
					m.selected[c.UUID] = struct{}{}
				}
			}
		}

	case "y":
		if len(m.selected) > 0 {
			m.clipboard = []card.Card{}
			m.isCut = false
			for _, col := range m.board.Columns {
				for _, c := range col.Cards {
					if _, ok := m.selected[c.UUID]; ok {
						m.clipboard = append(m.clipboard, c)
					}
				}
			}
			m.selected = make(map[string]struct{})
		}

	case "d":
		if len(m.selected) > 0 {
			m.clipboard = []card.Card{}
			m.isCut = true
			for _, col := range m.board.Columns {
				for _, c := range col.Cards {
					if _, ok := m.selected[c.UUID]; ok {
						m.clipboard = append(m.clipboard, c)
					}
				}
			}
			m.selected = make(map[string]struct{})
		}

	case "p":
		if len(m.clipboard) > 0 {
			destCol := &m.board.Columns[m.focusedColumn]
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
				destCol.Cards = append(destCol.Cards, m.clipboard...)
			} else {
				for _, c := range m.clipboard {
					newCard, err := fs.CopyCard(c, *destCol)
					if err == nil {
						destCol.Cards = append(destCol.Cards, newCard)
					}
				}
			}

			fs.WriteBoard(m.board)
			m.clipboard = []card.Card{}
			m.isCut = false
		}
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

		currentCol.Cards = append(currentCol.Cards, newCard)

		if err := fs.WriteBoard(m.board); err != nil {
			return nil
		}

		m.focusedCard = len(currentCol.Cards)
	}

	return nil
}

func (m *Model) clampFocusedCard() {
	if len(m.board.Columns) == 0 {
		m.focusedCard = 0
		return
	}
	maxIndex := m.board.Columns[m.focusedColumn].CardCount()

	if m.focusedCard < 0 {
		m.focusedCard = 0
	}
	if m.focusedCard > maxIndex {
		m.focusedCard = maxIndex
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

	if m.focusedCard == 0 {
		m.scrollOffset = 0
		return
	}

	focusedIdx := m.focusedCard - 1

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
