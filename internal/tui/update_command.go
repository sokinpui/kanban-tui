// internal/tui/update_command.go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"kanban/internal/card"
)

func (m *Model) updateCommandMode(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	prevVal := m.textInput.Value()

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.Type {
		case tea.KeyEscape, tea.KeyCtrlC:
			m.mode = normalMode
			m.textInput.Blur()
			m.createCardMode = "prepend"
			m.selected = make(map[string]struct{})
			m.clipboard = []card.Card{}
			m.isCut = false
			m.completionMatches = nil
			m.completionIndex = -1
			return nil
		case tea.KeyEnter:
			commandStr := m.textInput.Value()
			m.lastCommand = commandStr
			cmd = m.ExecuteCommand(commandStr)
			if m.mode == commandMode {
				m.mode = normalMode
			}
			m.textInput.Blur()
			m.completionMatches = nil
			m.completionIndex = -1
			return cmd
		case tea.KeyTab:
			m.cycleCompletion()
			return nil
		}
	}
	m.textInput, cmd = m.textInput.Update(msg)
	if m.textInput.Value() != prevVal {
		m.updateCompletions()
	}
	return cmd
}

func (m *Model) updateCompletions() {
	inputValue := m.textInput.Value()
	parts := strings.Split(inputValue, " ")

	if len(parts) == 0 {
		m.completionMatches = nil
		m.completionIndex = -1
		return
	}

	var candidates []string
	wordToComplete := parts[len(parts)-1]
	isCompletingArgument := len(parts) > 1 || (len(parts) == 1 && strings.HasSuffix(inputValue, " "))

	if !isCompletingArgument {
		for name := range commandRegistry {
			candidates = append(candidates, name)
		}
	} else {
		command := parts[0]
		if strings.HasSuffix(command, "!") {
			command = strings.TrimSuffix(command, "!")
		}
		if cmdInfo, ok := commandRegistry[command]; ok && cmdInfo.getCompletions != nil {
			// Why: We pass only the arguments to the completion function, not the command itself.
			argStr := ""
			if len(parts) > 1 {
				argStr = strings.Join(parts[1:], " ")
			}
			candidates = cmdInfo.getCompletions(argStr)
		}
	}

	if len(candidates) == 0 {
		m.completionMatches = nil
		m.completionIndex = -1
		return
	}

	var matches []string
	for _, c := range candidates {
		if strings.HasPrefix(c, strings.ToLower(wordToComplete)) {
			matches = append(matches, c)
		}
	}

	if len(matches) == 0 {
		m.completionMatches = nil
		m.completionIndex = -1
		return
	}

	m.completionMatches = matches
	m.completionIndex = -1
}

func (m *Model) cycleCompletion() {
	if len(m.completionMatches) == 0 {
		return
	}

	m.completionIndex++
	if m.completionIndex >= len(m.completionMatches) {
		m.completionIndex = 0
	}

	nextMatch := m.completionMatches[m.completionIndex]

	inputValue := m.textInput.Value()
	parts := strings.Split(inputValue, " ")
	prefixParts := parts[:len(parts)-1]
	newValue := strings.Join(append(prefixParts, nextMatch), " ")

	m.textInput.SetValue(newValue)
	m.textInput.SetCursor(len(newValue))
}
