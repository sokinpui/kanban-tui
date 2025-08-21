package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateSearchMode(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	prevVal := m.textInput.Value()

	m.textInput, cmd = m.textInput.Update(msg)

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.mode = normalMode
			m.textInput.Blur()
			m.textInput.SetValue("")
			m.searchResults = []searchResult{}
			return nil
		case tea.KeyEnter:
			query := m.textInput.Value()
			if query == "" && m.lastSearchQuery != "" {
				query = m.lastSearchQuery
				m.textInput.SetValue(query)
			}
			m.performSearch()
			m.lastSearchQuery = query
			m.lastSearchDirection = m.textInput.Prompt
			cmd = m.jumpToFirstResult(true)
			m.mode = normalMode
			m.textInput.Blur()
			return cmd
		}
	}

	if m.textInput.Value() != prevVal {
		m.performSearch()
		// Don't show "not found" message during incremental search
		if len(m.searchResults) > 0 {
			m.jumpToFirstResult(false)
		}
	}

	return cmd
}
