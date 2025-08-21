// internal/tui/fzf.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"kanban/internal/card"
)

type fzfCardSelectedMsg struct{ card card.Card }
type fzfCancelledMsg struct{}

type FzfItem struct {
	Card     card.Card
	ColTitle string
}

type itemSource []FzfItem

func (s itemSource) String(i int) string {
	return s[i].Card.Title
}

func (s itemSource) Len() int {
	return len(s)
}

var (
	fzfPopupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))

	fzfPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205"))

	fzfSelectedItemStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("236")).
				Foreground(lipgloss.Color("229"))

	fzfMatchedCharStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Underline(true)
)

type FZFModel struct {
	textinput   textinput.Model
	viewport    viewport.Model
	items       itemSource
	matches     fuzzy.Matches
	selectedIndex int
	width       int
	height      int
	ready       bool
}

func NewFZFModel() FZFModel {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "Find a card..."
	ti.PromptStyle = fzfPromptStyle
	ti.Focus()

	return FZFModel{
		textinput: ti,
	}
}

func (m *FZFModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.ready = true

	popupWidth := int(float64(w) * 0.8)
	if popupWidth > 120 {
		popupWidth = 120
	}
	popupHeight := int(float64(h) * 0.6)

	m.textinput.Width = popupWidth - 4 // Padding
	m.viewport.Width = popupWidth
	m.viewport.Height = popupHeight - 2 // Title and prompt
}

func (m *FZFModel) SetItems(items []FzfItem) {
	m.items = items
	m.filter()
}

func (m FZFModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m FZFModel) Focus() tea.Cmd {
	m.textinput.Focus()
	m.filter()
	return textinput.Blink
}

func (m FZFModel) Blur() {
	m.textinput.Blur()
	m.textinput.SetValue("")
}

func (m FZFModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			return m, func() tea.Msg { return fzfCancelledMsg{} }

		case tea.KeyEnter:
			if len(m.matches) > 0 {
				selectedItem := m.items[m.matches[m.selectedIndex].Index]
				return m, func() tea.Msg { return fzfCardSelectedMsg{card: selectedItem.Card} }
			}
			return m, func() tea.Msg { return fzfCancelledMsg{} }

		case tea.KeyDown, tea.KeyCtrlN:
			if m.selectedIndex < len(m.matches)-1 {
				m.selectedIndex++
			} else {
				m.selectedIndex = 0
			}

		case tea.KeyUp, tea.KeyCtrlP:
			if m.selectedIndex > 0 {
				m.selectedIndex--
			} else {
				m.selectedIndex = len(m.matches) - 1
			}
		}
	}

	m.textinput, cmd = m.textinput.Update(msg)
	cmds = append(cmds, cmd)

	if m.textinput.Value() != "" {
		m.filter()
	} else {
		m.matches = nil
	}
	m.viewport.SetContent(m.renderResults())
	m.ensureSelectedItemVisible()

	return m, tea.Batch(cmds...)
}

func (m *FZFModel) filter() {
	m.matches = fuzzy.FindFrom(m.textinput.Value(), m.items)
	m.selectedIndex = 0
}

func (m *FZFModel) ensureSelectedItemVisible() {
	m.viewport.SetYOffset(m.selectedIndex)
}

func (m FZFModel) renderResults() string {
	var b strings.Builder
	for i, match := range m.matches {
		item := m.items[match.Index]

		line := ""
		if i == m.selectedIndex {
			line += "> "
		} else {
			line += "  "
		}

		title := ""
		matchedIndexes := make(map[int]struct{})
		for _, idx := range match.MatchedIndexes {
			matchedIndexes[idx] = struct{}{}
		}

		for charIdx, char := range item.Card.Title {
			if _, ok := matchedIndexes[charIdx]; ok {
				title += fzfMatchedCharStyle.Render(string(char))
			} else {
				title += string(char)
			}
		}

		line += fmt.Sprintf("%s [%s]", title, item.ColTitle)

		if i == m.selectedIndex {
			b.WriteString(fzfSelectedItemStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteRune('\n')
	}
	return b.String()
}

func (m FZFModel) View() string {
	if !m.ready {
		return ""
	}

	popupWidth := int(float64(m.width) * 0.8)
	if popupWidth > 120 {
		popupWidth = 120
	}
	popupHeight := int(float64(m.height) * 0.6)

	m.viewport.Width = popupWidth - 4
	m.viewport.Height = popupHeight - 3

	m.viewport.SetContent(m.renderResults())

	title := "Find Card"
	prompt := m.textinput.View()

	content := lipgloss.JoinVertical(lipgloss.Left, title, m.viewport.View(), prompt)
	popup := fzfPopupStyle.Width(popupWidth).Height(popupHeight).Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}
