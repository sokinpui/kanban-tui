// internal/tui/view.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"kanban/internal/card"
	"kanban/internal/column"
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
)

func renderBoard(m *Model) string {
	if m.width <= 0 || len(m.board.Columns) == 0 {
		return ""
	}

	numColumns := len(m.board.Columns)
	baseColumnWidth := m.width / numColumns
	remainder := m.width % numColumns

	var renderedColumns []string
	for i, col := range m.board.Columns {
		colWidth := baseColumnWidth
		if i < remainder {
			colWidth++
		}
		renderedColumns = append(renderedColumns, renderColumn(col, m, i, colWidth))
	}
	boardView := lipgloss.JoinHorizontal(lipgloss.Top, renderedColumns...)
	return boardView
}

func renderColumn(c column.Column, m *Model, columnIndex int, width int) string {
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
	cardAreaHeight := m.cardAreaHeight()
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

		separatorHeight := 0
		if len(renderedCards) > 0 {
			separatorHeight = 1
		}

		if currentHeight+cardHeight+separatorHeight > cardAreaHeight {
			break
		}

		renderedCards = append(renderedCards, renderedCard)
		currentHeight += cardHeight + separatorHeight
	}

	cards := strings.Join(renderedCards, "\n")
	columnContent := lipgloss.JoinVertical(lipgloss.Left, renderedHeader, cards)

	return columnStyle.Copy().Width(width).Render(columnContent)
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

	return style.Copy().Width(contentWidth).Render(c.Title)
}

func renderStatusBar(m *Model) string {
	switch m.mode {
	case commandMode:
		return m.textInput.View()
	case visualMode:
		return statusBarStyle.Copy().Width(m.width).Render("-- VISUAL --")
	default:
		return ""
	}
}
