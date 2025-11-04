package server

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins
}

type Server struct {
	hub *Hub
}

func Start(addr string) error {
	hub := newHub()
	go hub.run()

	watcher, err := newWatcher(hub)
	if err != nil {
		return err
	}
	defer watcher.Close()
	go watcher.run()

	server := &Server{hub: hub}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/board", server.handleGetBoard())
	mux.HandleFunc("/api/columns/", server.handleCreateCard()) // Simplified routing
	// Add routes for other actions here
	mux.HandleFunc("/ws", server.serveWs())

	log.Printf("Server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}
