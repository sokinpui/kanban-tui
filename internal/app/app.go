package app

import (
	"github.com/charmbracelet/bubbletea"
	"kanban/internal/board"
	"kanban/internal/card"
	"kanban/internal/column"
	"kanban/internal/tui"
)

func Run() error {
	initialBoard := createInitialBoard()
	model := tui.NewModel(initialBoard)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func createInitialBoard() board.Board {
	return board.New(
		[]column.Column{
			column.New("notes",
				card.New("Available sport API"),
				card.New("build ros2 in zsh"),
				card.New("documents"),
				card.New("software interface"),
				card.New("IP of the dog"),
				card.New("resources & link"),
				card.New("what Am I DOING"),
				card.New("what is DDS"),
				card.New("python sdk SDK"),
			),
			column.New("planned",
				card.New("play with ros2"),
				card.New("check other API"),
				card.New("use ros to build some simple service"),
				card.New("get camera data"),
			),
			column.New("working"),
			column.New("done",
				card.New("#quick explore all sport/movement related high level api"),
			),
			column.New("testing"),
		},
	)
}
