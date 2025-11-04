package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"kanban/internal/server"
)

var serverCmd = &cobra.Command{
	Use:   "server [path]",
	Short: "Run the kanban server",
	Long:  `Run the kanban server in the specified directory, or the current directory if none is provided.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			if err := os.Chdir(args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error changing to directory %s: %v\n", args[0], err)
				os.Exit(1)
			}
		}

		addr, _ := cmd.Flags().GetString("addr")
		fmt.Printf("Starting kanban server on %s\n", addr)
		if err := server.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serverCmd.Flags().String("addr", ":8080", "Address for the server to listen on")
}
