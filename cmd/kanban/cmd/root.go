package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"kanban/internal/client"
	"kanban/internal/core"
	"kanban/internal/tui"
)

var (
	host string
)

var rootCmd = &cobra.Command{
	Use:   "kanban",
	Short: "A personal kanban board for your terminal",
	Long:  `A personal kanban board for your terminal, inspired by vim-kanban.`,
	Run:   run,
}

func init() {
	rootCmd.Flags().StringVar(&host, "host", "http://localhost:8080", "Address of the kanban server")
	rootCmd.AddCommand(serverCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) {
	c := client.New(host)
	board, err := c.LoadBoard()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not connect to server at %s. Is it running?\nError: %v\n", host, err)
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
			if err := core.CreateSampleBoard(&board); err != nil {
				fmt.Fprintf(os.Stderr, "could not create sample board: %v\n", err)
				os.Exit(1)
			}
			board, err = core.LoadBoard()
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

	state, err := core.LoadState()
	if err != nil {
		// Non-fatal, we can continue with defaults
		fmt.Fprintf(os.Stderr, "could not load state: %v\n", err)
	}

	model := tui.NewModel(c, board, &state)
	p := tea.NewProgram(&model, tea.WithAltScreen())
	model.SetProgram(p)
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
		if err := core.SaveState(s.FocusedColumn, s.FocusedCard, s.DoneColumn, s.ShowHidden); err != nil {
			fmt.Fprintf(os.Stderr, "could not save state: %v\n", err)
			os.Exit(1)
		}
	}
}
