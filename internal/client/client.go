package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"kanban/internal/core"
	"kanban/internal/models"
)

type BoardRefreshedMsg struct {
	Board models.Board
}
type ServerErrorMsg struct{ Err error }

// Client is a client for the kanban server.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.baseURL+"/api"+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

func (c *Client) LoadBoard() (models.Board, error) {
	var board models.Board
	resp, err := c.doRequest("GET", "/board", nil)
	if err != nil {
		return board, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return board, fmt.Errorf("server returned non-200 status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&board); err != nil {
		return board, err
	}
	return board, nil
}

func (c *Client) WriteBoard(b models.Board) error {
	// This is now a server-side concern, triggered by other actions.
	// The client no longer tells the server *how* to write the board.
	return nil
}

func (c *Client) LoadCard(path string) (models.Card, error) {
	// This is also a server-side detail. The client should ask for a card by ID.
	// For now, we'll rely on the full board sync.
	return models.Card{}, fmt.Errorf("LoadCard is not implemented in client-server model")
}

func (c *Client) WriteCard(card models.Card) error {
	resp, err := c.doRequest("PUT", fmt.Sprintf("/cards/%s", card.UUID), card)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %s", resp.Status)
	}
	return nil
}

func (c *Client) CreateCard(col models.Column, title string) (models.Card, error) {
	var newCard models.Card
	body := map[string]string{"title": title}
	resp, err := c.doRequest("POST", fmt.Sprintf("/columns/%s/cards", col.Title), body)
	if err != nil {
		return newCard, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return newCard, fmt.Errorf("server returned non-201 status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&newCard); err != nil {
		return newCard, err
	}
	return newCard, nil
}

func (c *Client) MoveCard(card *models.Card, destCol models.Column) error {
	body := map[string]string{"destColumn": destCol.Title}
	resp, err := c.doRequest("PATCH", fmt.Sprintf("/cards/%s/move", card.UUID), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %s", resp.Status)
	}
	return nil
}

func (c *Client) CopyCard(card models.Card, destCol models.Column) (models.Card, error) {
	var newCard models.Card
	body := map[string]string{"destColumn": destCol.Title}
	resp, err := c.doRequest("POST", fmt.Sprintf("/cards/%s/copy", card.UUID), body)
	if err != nil {
		return newCard, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return newCard, fmt.Errorf("server returned non-201 status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&newCard); err != nil {
		return newCard, err
	}
	return newCard, nil
}

func (c *Client) CreateColumn(name string) (models.Column, error) {
	var newCol models.Column
	body := map[string]string{"name": name}
	resp, err := c.doRequest("POST", "/columns", body)
	if err != nil {
		return newCol, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return newCol, fmt.Errorf("server returned non-201 status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&newCol); err != nil {
		return newCol, err
	}
	return newCol, nil
}

func (c *Client) DeleteColumn(col models.Column) error {
	resp, err := c.doRequest("DELETE", fmt.Sprintf("/columns/%s", col.Title), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned non-204 status: %s", resp.Status)
	}
	return nil
}

func (c *Client) RenameColumn(col *models.Column, newName string) error {
	body := map[string]string{"newName": newName}
	resp, err := c.doRequest("PATCH", fmt.Sprintf("/columns/%s/rename", col.Title), body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %s", resp.Status)
	}
	return nil
}

func (c *Client) SynchronizeBoard(b models.Board) error {
	// This is now a server-side concern.
	return nil
}

func (c *Client) LoadState() (core.AppState, error) {
	// State is now managed locally by the client, not synced with the server.
	return core.LoadState()
}

func (c *Client) SaveState(focusedColumn, focusedCard int, doneColumn string, showHidden bool) error {
	// State is now managed locally by the client.
	return core.SaveState(focusedColumn, focusedCard, doneColumn, showHidden)
}

func (c *Client) FlushTrash(trash []models.Card) error {
	// This is a server-side concern.
	return nil
}

func (c *Client) ConnectWebSocket(program *tea.Program) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		program.Send(ServerErrorMsg{Err: fmt.Errorf("invalid base URL: %w", err)})
		return
	}
	scheme := "ws"
	if u.Scheme == "https" {
		scheme = "wss"
	}
	wsURL := url.URL{Scheme: scheme, Host: u.Host, Path: "/ws"}

	go func() {
		for {
			conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
			if err != nil {
				program.Send(ServerErrorMsg{Err: fmt.Errorf("websocket dial error: %w", err)})
				time.Sleep(5 * time.Second) // Retry connection
				continue
			}
			defer conn.Close()

			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					program.Send(ServerErrorMsg{Err: fmt.Errorf("websocket read error: %w", err)})
					break // Reconnect
				}

				if string(message) == "board_updated" {
					newBoard, err := c.LoadBoard()
					if err != nil {
						program.Send(ServerErrorMsg{Err: fmt.Errorf("failed to reload board after update: %w", err)})
					} else {
						program.Send(BoardRefreshedMsg{Board: newBoard})
					}
				}
			}
		}
	}()
}
