package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateSearchMode(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.mode = normalMode
			m.textInput.Blur()
			m.textInput.SetValue("")
			return nil
		case tea.KeyEnter:
			m.performSearch()
			cmd = m.jumpToFirstResult()
			m.mode = normalMode
			m.textInput.Blur()
			return cmd
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	return cmd
}
