package main

import (
	"fmt"
	"os"

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
		if err := fs.CreateSampleBoard(&board); err != nil {
			fmt.Fprintf(os.Stderr, "could not create sample board: %v\n", err)
			os.Exit(1)
		}
		board, err = fs.LoadBoard()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not load board after creating sample: %v\n", err)
			os.Exit(1)
		}
	}

	state, err := fs.LoadState()
	if err != nil {
		// Non-fatal, we can continue with defaults
		fmt.Fprintf(os.Stderr, "could not load state: %v\n", err)
	}

	model := tui.NewModel(board, state.FocusedColumn, state.FocusedCard)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if m, ok := finalModel.(*tui.Model); ok {
		if err := fs.SaveState(m.FocusedColumn(), m.FocusedCard()); err != nil {
			fmt.Fprintf(os.Stderr, "could not save state: %v\n", err)
			os.Exit(1)
		}
	}
}
