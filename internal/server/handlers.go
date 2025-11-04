package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"kanban/internal/core"
	"kanban/internal/models"
)

func (s *Server) handleGetBoard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		board, err := core.LoadBoard()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(board)
	}
}

func (s *Server) handleCreateCard() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		colName := strings.TrimPrefix(r.URL.Path, "/api/columns/")
		colName = strings.TrimSuffix(colName, "/cards")

		var body struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		board, err := core.LoadBoard()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var targetCol *models.Column
		for i := range board.Columns {
			if board.Columns[i].Title == colName {
				targetCol = &board.Columns[i]
				break
			}
		}

		if targetCol == nil {
			http.Error(w, "column not found", http.StatusNotFound)
			return
		}

		card, err := core.CreateCard(*targetCol, body.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		targetCol.Cards = append(targetCol.Cards, card)
		if err := core.WriteBoard(board); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		s.hub.broadcast <- []byte("board_updated")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(card)
	}
}

// Add other handlers for MoveCard, CreateColumn, etc. in a similar fashion.
// For brevity, they are omitted here but would follow the same pattern:
// 1. Decode request.
// 2. Load the board state.
// 3. Perform the action using a core function.
// 4. Write the board state back to disk.
// 5. Broadcast "board_updated" to the hub.
// 6. Write the HTTP response.

func (s *Server) serveWs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		s.hub.register <- conn

		// This is a simple implementation. A more robust one would handle
		// client-side messages and pong responses.
		go func() {
			defer func() {
				s.hub.unregister <- conn
				conn.Close()
			}()
			for {
				if _, _, err := conn.NextReader(); err != nil {
					break
				}
			}
		}()
	}
}
