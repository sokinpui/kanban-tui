package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/fs"
	"kanban/internal/tui"
)

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "--main" {
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "error: --main requires at least one path argument")
				os.Exit(1)
			}
			paths := os.Args[2:]
			if err := fs.SetupMainBoard(paths); err != nil {
				fmt.Fprintf(os.Stderr, "error setting up main board: %v\n", err)
				os.Exit(1)
			}
		} else {
			targetPath := os.Args[1]
			info, err := os.Stat(targetPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error accessing path %s: %v\n", targetPath, err)
				os.Exit(1)
			}

			var dir string
			if info.IsDir() {
				dir = targetPath
			} else {
				if filepath.Base(targetPath) == fs.BoardFileName {
					dir = filepath.Dir(targetPath)
				} else {
					fmt.Fprintf(os.Stderr, "error: provided file must be named %s\n", fs.BoardFileName)
					os.Exit(1)
				}
			}

			if err := os.Chdir(dir); err != nil {
				fmt.Fprintf(os.Stderr, "error changing to directory %s: %v\n", dir, err)
				os.Exit(1)
			}
		}
	}

	board, err := fs.LoadBoard()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not load board: %v\n", err)
		os.Exit(1)
	}

	if len(board.Columns) == 0 {
		fmt.Print("No kanban board (kanban.md) found in the current directory.\nCreate a sample board? (y/N) ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not read response: %v\n", err)
			os.Exit(1)
		}

		if strings.ToLower(strings.TrimSpace(response)) == "y" {
			if err := fs.CreateSampleBoard(&board); err != nil {
				fmt.Fprintf(os.Stderr, "could not create sample board: %v\n", err)
				os.Exit(1)
			}
			board, err = fs.LoadBoard()
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not load board after creating sample: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Sample board created. Starting kanban...")
		} else {
			fmt.Println("Aborting.")
			os.Exit(0)
		}
	}

	state, err := fs.LoadState()
	if err != nil {
		// Non-fatal, we can continue with defaults
		fmt.Fprintf(os.Stderr, "could not load state: %v\n", err)
	}

	model := tui.NewModel(board, &state)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(*tui.Model); ok {
		if err := m.Cleanup(); err != nil {
			// Don't exit on cleanup error, just warn and continue to save state
			fmt.Fprintf(os.Stderr, "error during cleanup: %v\n", err)
		}

		s := m.State()
		if err := fs.SaveState(s.FocusedColumn, s.FocusedCard, s.DoneColumn, s.ShowHidden); err != nil {
			fmt.Fprintf(os.Stderr, "could not save state: %v\n", err)
			os.Exit(1)
		}
	}
}
