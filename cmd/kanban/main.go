package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"kanban/internal/fs"
	"kanban/internal/tui"
)

func main() {
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
		s := m.State()
		if err := fs.SaveState(s.FocusedColumn, s.FocusedCard, s.DoneColumn, s.ShowHidden); err != nil {
			fmt.Fprintf(os.Stderr, "could not save state: %v\n", err)
			os.Exit(1)
		}
	}
}
