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
			Margin(0, cardMarginHorizontal).
			Width(CardWidth)

	selectedCardStyle = cardStyle.Copy().
				BorderForeground(lipgloss.Color("220"))

	cutCardStyle = cardStyle.Copy().
			BorderForeground(lipgloss.Color("196"))

	focusedCardStyle = cardStyle.Copy().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("205"))

	columnStyle = lipgloss.NewStyle().
			Padding(0, columnPaddingHorizontal)
)

func renderBoard(m *Model) string {
	var renderedColumns []string
	for i, col := range m.board.Columns {
		renderedColumns = append(renderedColumns, renderColumn(col, m, i))
	}
	boardView := lipgloss.JoinHorizontal(lipgloss.Top, renderedColumns...)
	return boardView
}

func renderColumn(c column.Column, m *Model, columnIndex int) string {
	isColumnFocused := m.focusedColumn == columnIndex
	isHeaderFocused := isColumnFocused && m.focusedCard == 0

	header := fmt.Sprintf("%s %d", c.Title, c.CardCount())

	var renderedHeader string
	if isHeaderFocused {
		renderedHeader = focusedColumnHeaderStyle.Render(header)
	} else {
		renderedHeader = columnHeaderStyle.Render(header)
	}

	var renderedCards []string
	cardAreaHeight := m.cardAreaHeight()
	currentHeight := 0

	start := m.scrollOffset
	if start < 0 {
		start = 0
	}

	for i := start; i < len(c.Cards); i++ {
		crd := c.Cards[i]
		renderedCard := renderCard(crd, m, columnIndex, i)
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

	return columnStyle.Render(lipgloss.JoinVertical(lipgloss.Left, renderedHeader, cards))
}

func renderCard(c card.Card, m *Model, columnIndex, cardIndex int) string {
	isFocused := m.focusedColumn == columnIndex && m.focusedCard == cardIndex+1
	_, isSelected := m.selected[c.UUID]
	isMarkedForCut := m.isCardMarkedForCut(c.UUID)

	style := cardStyle.Copy()

	if isMarkedForCut {
		style = cutCardStyle
	} else if isSelected {
		style = selectedCardStyle
	}

	if isFocused {
		style = focusedCardStyle
	}

	return style.Render(c.Title)
}

func renderStatusBar(m *Model) string {
	if m.mode == commandMode {
		return m.textInput.View()
	}
	return ""
}
