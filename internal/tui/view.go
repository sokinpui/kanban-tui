package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"kanban/internal/board"
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
			Margin(0, 1, 1, 1)

	columnStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

func renderBoard(b board.Board) string {
	var renderedColumns []string
	for _, col := range b.Columns {
		renderedColumns = append(renderedColumns, renderColumn(col))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, renderedColumns...)
}

func renderColumn(c column.Column) string {
	header := fmt.Sprintf("%s %d", c.Title, c.CardCount())
	renderedHeader := columnHeaderStyle.Render(header)

	var renderedCards []string
	for _, crd := range c.Cards {
		renderedCards = append(renderedCards, renderCard(crd))
	}

	// Add empty cards to fill space, mimicking the original image
	if len(renderedCards) == 0 {
		for i := 0; i < 2; i++ {
			renderedCards = append(renderedCards, renderCard(card.Card{Title: ""}))
		}
	}

	cards := strings.Join(renderedCards, "\n")
	return columnStyle.Render(lipgloss.JoinVertical(lipgloss.Left, renderedHeader, cards))
}

func renderCard(c card.Card) string {
	return cardStyle.Render(c.Title)
}
