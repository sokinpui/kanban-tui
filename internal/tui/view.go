// internal/tui/view.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"kanban/internal/card"
	"kanban/internal/column"
	"kanban/internal/fs"
)

var (
	columnHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Padding(0, 1)

	focusedColumnHeaderStyle = columnHeaderStyle.Copy().
					Foreground(lipgloss.Color("231")).
					Background(lipgloss.Color("205"))

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Margin(0, cardMarginHorizontal)

	selectedCardStyle = cardStyle.Copy().
				BorderForeground(lipgloss.Color("220")).
				Background(lipgloss.Color("#3e6452")).
				Foreground(lipgloss.Color("231"))

	cutCardStyle = cardStyle.Copy().
			BorderForeground(lipgloss.Color("196"))

	focusedCardStyle = cardStyle.Copy().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("205"))

	focusedAndSelectedCardStyle = selectedCardStyle.Copy().
					Border(lipgloss.DoubleBorder()).
					BorderForeground(lipgloss.Color("205"))

	columnStyle = lipgloss.NewStyle().
			Padding(0, columnPaddingHorizontal)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("#3e6452"))

	statusModeNormal = lipgloss.NewStyle().
				Background(lipgloss.Color("41")). // Green
				Foreground(lipgloss.Color("232")). // Dark text
				Padding(0, 1)

	statusModeVisual = lipgloss.NewStyle().
				Background(lipgloss.Color("98")). // Purple
				Foreground(lipgloss.Color("232")).
				Padding(0, 1)

	statusModeCommand = lipgloss.NewStyle().
				Background(lipgloss.Color("214")). // Yellow
				Foreground(lipgloss.Color("232")).
				Padding(0, 1)

	statusInfo = lipgloss.NewStyle().
			Background(lipgloss.Color("236")).
			Foreground(lipgloss.Color("250"))

	commandBarTextStyle = lipgloss.NewStyle().Bold(true)

	searchHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("220")).
				Foreground(lipgloss.Color("232"))
)

func renderBoard(m *Model, height int) string {
	if m.width <= 0 || len(m.displayColumns) == 0 || height <= 0 {
		return ""
	}

	numColumns := len(m.displayColumns)
	separator := "│"
	numSeparators := numColumns - 1
	if numSeparators < 0 {
		numSeparators = 0
	}

	availableWidth := m.width - numSeparators
	baseColumnWidth := availableWidth / numColumns
	remainder := availableWidth % numColumns

	var renderedColumns []string
	for i, col := range m.displayColumns {
		colWidth := baseColumnWidth
		if i < remainder {
			colWidth++
		}
		renderedColumns = append(renderedColumns, renderColumn(*col, m, i, colWidth, height))
	}

	var parts []string
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	for i, s := range renderedColumns {
		parts = append(parts, s)
		if i < len(renderedColumns)-1 {
			parts = append(parts, separatorStyle.Render(separator))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func renderColumn(c column.Column, m *Model, columnIndex int, width int, height int) string {
	isColumnFocused := m.focusedColumn == columnIndex
	isHeaderFocused := isColumnFocused && m.currentFocusedCard() == 0

	header := fmt.Sprintf("%s %d", c.Title, c.CardCount())

	headerStyle := columnHeaderStyle
	if isHeaderFocused {
		headerStyle = focusedColumnHeaderStyle
	}

	headerContentWidth := width - columnStyle.GetHorizontalPadding() - headerStyle.GetHorizontalPadding()
	renderedHeader := headerStyle.Copy().Width(headerContentWidth).Render(header)

	var renderedCards []string
	headerHeight := lipgloss.Height(renderedHeader)
	cardAreaHeight := height - headerHeight
	currentHeight := 0
	cardContentW := m.cardContentWidth(width)

	start := m.scrollOffset
	if start < 0 {
		start = 0
	}

	for i := start; i < len(c.Cards); i++ {
		crd := c.Cards[i]
		renderedCard := renderCard(crd, m, columnIndex, i, cardContentW)
		cardHeight := lipgloss.Height(renderedCard)
		if currentHeight+cardHeight > cardAreaHeight {
			break
		}

		renderedCards = append(renderedCards, renderedCard)
		currentHeight += cardHeight
	}

	cards := strings.Join(renderedCards, "\n")
	columnContent := lipgloss.JoinVertical(lipgloss.Left, renderedHeader, cards)

	return columnStyle.Copy().Width(width).Height(height).Render(columnContent)
}

func renderCard(c card.Card, m *Model, columnIndex, cardIndex int, contentWidth int) string {
	isFocused := m.focusedColumn == columnIndex && m.currentFocusedCard() == cardIndex+1
	_, isSelected := m.selected[c.UUID]
	isMarkedForCut := m.isCardMarkedForCut(c.UUID)

	style := cardStyle.Copy()

	if isFocused && isSelected {
		style = focusedAndSelectedCardStyle
	} else if isFocused {
		style = focusedCardStyle
	} else if isMarkedForCut {
		style = cutCardStyle
	} else if isSelected {
		style = selectedCardStyle
	}

	title := c.Title
	var query string
	if m.mode == searchMode {
		query = m.textInput.Value()
	}
	if query != "" {
		lowerTitle := strings.ToLower(title)
		lowerQuery := strings.ToLower(query)

		if idx := strings.Index(lowerTitle, lowerQuery); idx != -1 {
			pre := title[:idx]
			match := title[idx : idx+len(query)]
			post := title[idx+len(query):]

			highlightedMatch := searchHighlightStyle.Render(match)
			title = lipgloss.JoinHorizontal(lipgloss.Top, pre, highlightedMatch, post)
		}
	}

	if c.HasContent() && !isSelected && m.mode != searchMode {
		style = style.Foreground(lipgloss.Color("81"))
	}

	return style.Copy().Width(contentWidth).Render(title)
}

func renderStatusBar(m *Model) string {
	var modeStr string
	var modeStyle lipgloss.Style

	switch m.mode {
	case visualMode:
		modeStr = "VISUAL"
		modeStyle = statusModeVisual
	case commandMode, searchMode:
		modeStr = "COMMAND"
		modeStyle = statusModeCommand
	default: // normalMode and others
		modeStr = "NORMAL"
		modeStyle = statusModeNormal
	}

	renderedMode := modeStyle.Render(modeStr)
	fileInfo := statusInfo.Render(" " + fs.BoardFileName + " ")

	remainingWidth := m.width - lipgloss.Width(renderedMode) - lipgloss.Width(fileInfo)
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	filler := statusInfo.Render(strings.Repeat(" ", remainingWidth))

	statusLine := lipgloss.JoinHorizontal(lipgloss.Left, renderedMode, fileInfo, filler)

	var commandLine string
	if m.statusMessage != "" {
		commandLine = statusBarStyle.Copy().Width(m.width).Render(m.statusMessage)
	} else {
		switch m.mode {
		case visualMode:
			commandLine = commandBarTextStyle.Render("-- VISUAL --")
		case commandMode, searchMode:
			commandLine = m.textInput.View()
		default:
			commandLine = "" // Render an empty line to reserve space
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, statusLine, commandLine)
}
