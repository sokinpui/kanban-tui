package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	// "kanban/internal/board"
	"kanban/internal/card"
	"kanban/internal/column"
)

var (
	columnHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Padding(0, 1)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			Margin(0, 1).
			Width(22)

	focusedCardStyle = cardStyle.Copy().
				BorderForeground(lipgloss.Color("205"))

	expandedCardStyle = focusedCardStyle.Copy().
				UnsetWidth()

	columnStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

func renderBoard(m Model) string {
	var renderedColumns []string
	for i, col := range m.board.Columns {
		renderedColumns = append(renderedColumns, renderColumn(col, m, i))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, renderedColumns...)
}

func renderColumn(c column.Column, m Model, columnIndex int) string {
	header := fmt.Sprintf("%s %d", c.Title, c.CardCount())
	renderedHeader := columnHeaderStyle.Render(header)

	var renderedCards []string
	for i, crd := range c.Cards {
		renderedCards = append(renderedCards, renderCard(crd, m, columnIndex, i))
	}

	cards := strings.Join(renderedCards, "\n")
	return columnStyle.Render(lipgloss.JoinVertical(lipgloss.Left, renderedHeader, cards))
}

func renderCard(c card.Card, m Model, columnIndex, cardIndex int) string {
	isFocused := m.focusedColumn == columnIndex && m.focusedCard == cardIndex
	if isFocused && m.mode == insertMode {
		return expandedCardStyle.Render(m.textInput.View())
	}

	style := cardStyle
	if isFocused {
		style = focusedCardStyle
	}

	return style.Render(c.Title)
}
