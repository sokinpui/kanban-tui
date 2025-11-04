package server

import (
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"kanban/internal/core"
)

type Watcher struct {
	watcher *fsnotify.Watcher
	hub     *Hub
}

func newWatcher(hub *Hub) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher: fsWatcher,
		hub:     hub,
	}

	// Watch the current directory for kanban.md
	if err := w.watcher.Add("."); err != nil {
		return nil, err
	}
	// Watch the .kanban directory for card changes
	if _, err := os.Stat(core.DataDirName); !os.IsNotExist(err) {
		err := filepath.Walk(core.DataDirName, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return w.watcher.Add(path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

func (w *Watcher) run() {
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// We can be more granular, but for now, any write event triggers a refresh.
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Printf("File modified: %s, broadcasting update", event.Name)
				w.hub.broadcast <- []byte("board_updated")
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Println("watcher error:", err)
		}
	}
}

func (w *Watcher) Close() {
	w.watcher.Close()
}
