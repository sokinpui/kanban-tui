// internal/tui/editor.go
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/board"
	"kanban/internal/fs"
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

type boardSwitchedMsg struct {
	board board.Board
	state fs.AppState
	err   error
}

func switchToBoardCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return boardSwitchedMsg{err: fmt.Errorf("could not resolve home dir: %w", err)}
			}
			path = filepath.Join(home, path[2:])
		}

		info, err := os.Stat(path)
		if err != nil {
			return boardSwitchedMsg{err: fmt.Errorf("path not found: %s", path)}
		}
		if info.IsDir() {
			return boardSwitchedMsg{err: fmt.Errorf("link is a directory, not a kanban.md file")}
		}
		if filepath.Base(path) != fs.BoardFileName {
			return boardSwitchedMsg{err: fmt.Errorf("linked file is not named %s", fs.BoardFileName)}
		}

		dir := filepath.Dir(path)
		if err := os.Chdir(dir); err != nil {
			return boardSwitchedMsg{err: fmt.Errorf("could not change to directory %s: %w", dir, err)}
		}

		newBoard, err := fs.LoadBoard()
		if err != nil {
			return boardSwitchedMsg{err: fmt.Errorf("could not load board: %w", err)}
		}

		newState, err := fs.LoadState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not load state for new board: %v\n", err)
		}

		return boardSwitchedMsg{board: newBoard, state: newState}
	}
}
