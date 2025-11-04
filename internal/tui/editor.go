// internal/tui/editor.go
package tui

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
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
