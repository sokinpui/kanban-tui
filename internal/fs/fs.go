// internal/fs/fs.go
package fs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
	"kanban/internal/board"
	"kanban/internal/card"
	"kanban/internal/column"
)

const (
	BoardFileName  = "kanban.md"
	DataDirName    = ".kanban"
	StateFileName  = "state.json"
	frontMatterSep = "---\n"
	ArchiveColumnName = "Archived"
)

type AppState struct {
	FocusedColumn int `json:"focused_column"`
	FocusedCard   int `json:"focused_card"`
	DoneColumn    string `json:"done_column,omitempty"`
	ShowHidden    bool   `json:"show_hidden,omitempty"`
}

var cardLinkRegex = regexp.MustCompile(`\s*-\s*\[(.*?)\]\((.*?)\)`)

func LoadBoard() (board.Board, error) {
	wd, err := os.Getwd()
	if err != nil {
		return board.Board{}, err
	}
	b := board.New(wd, []column.Column{})

	f, err := os.Open(BoardFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return board.Board{}, err
	}
	defer f.Close()

	allCols := make([]column.Column, 0)
	var currentColumn *column.Column

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			if currentColumn != nil {
				allCols = append(allCols, *currentColumn)
			}
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			currentColumn = &column.Column{
				Title: title,
				Path:  filepath.Join(DataDirName, title),
				Cards: []card.Card{},
			}
		} else if currentColumn != nil && cardLinkRegex.MatchString(line) {
			matches := cardLinkRegex.FindStringSubmatch(line)
			if len(matches) == 3 {
				cardPath := matches[2]
				c, err := LoadCard(cardPath)
				if err == nil {
					currentColumn.Cards = append(currentColumn.Cards, c)
				}
			}
		}
	}

	if currentColumn != nil {
		allCols = append(allCols, *currentColumn)
	}

	if err := scanner.Err(); err != nil {
		return board.Board{}, err
	}

	displayCols := make([]column.Column, 0)
	var archivedCol *column.Column

	for i := range allCols {
		if allCols[i].Title == ArchiveColumnName {
			archivedCol = &allCols[i]
		} else {
			displayCols = append(displayCols, allCols[i])
		}
	}

	b.Columns = displayCols
	if archivedCol != nil {
		b.Archived = *archivedCol
	} else {
		b.Archived = column.New(ArchiveColumnName, filepath.Join(DataDirName, ArchiveColumnName))
	}

	return b, nil
}

func WriteBoard(b board.Board) error {
	var builder strings.Builder

	allColumns := make([]column.Column, len(b.Columns))
	copy(allColumns, b.Columns)
	if len(b.Archived.Cards) > 0 {
		allColumns = append(allColumns, b.Archived)
	}

	for i, col := range allColumns {
		builder.WriteString(fmt.Sprintf("# %s\n", col.Title))
		for _, crd := range col.Cards {
			builder.WriteString(fmt.Sprintf("- [%s](%s)\n", crd.Title, crd.Path))
		}
		if i < len(allColumns)-1 {
			builder.WriteString("\n")
		}
	}

	return os.WriteFile(BoardFileName, []byte(builder.String()), 0644)
}

func CreateSampleBoard(b *board.Board) error {
	sampleCols := []string{"Notes", "Planned", "WIP", "Done"}
	var columns []column.Column

	if err := os.Mkdir(DataDirName, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	for _, colName := range sampleCols {
		colPath := filepath.Join(DataDirName, colName)
		if err := os.Mkdir(colPath, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		col := column.New(colName, colPath)
		columns = append(columns, col)
	}
	b.Columns = columns
	return WriteBoard(*b)
}

func LoadCard(path string) (card.Card, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return card.Card{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return card.Card{}, err
	}

	parts := strings.SplitN(string(data), frontMatterSep, 3)
	if len(parts) < 3 {
		return card.Card{}, fmt.Errorf("invalid markdown format: missing front matter")
	}

	var c card.Card
	if err := yaml.Unmarshal([]byte(parts[1]), &c); err != nil {
		return card.Card{}, err
	}

	c.Content = strings.TrimSpace(parts[2])
	c.Path = path
	c.UUID = strings.TrimSuffix(filepath.Base(path), ".md")
	c.Size = fileInfo.Size()

	return c, nil
}

func CreateCard(col column.Column, title string) (card.Card, error) {
	id := uuid.New()
	now := time.Now()

	c := card.Card{
		UUID:       id.String(),
		Title:      title,
		CreatedAt:  now,
		ModifiedAt: now,
		Path:       filepath.Join(col.Path, id.String()+".md"),
	}

	if err := WriteCard(c); err != nil {
		return card.Card{}, err
	}

	fileInfo, err := os.Stat(c.Path)
	if err != nil {
		// This should not happen as we just wrote the file
		return c, nil
	}
	c.Size = fileInfo.Size()

	return c, nil
}

func WriteCard(c card.Card) error {
	c.ModifiedAt = time.Now()

	frontMatter, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s%s%s\n%s\n", frontMatterSep, string(frontMatter), frontMatterSep, c.Content)

	return os.WriteFile(c.Path, []byte(content), 0644)
}

func MoveCard(c *card.Card, destCol column.Column) error {
	newPath := filepath.Join(destCol.Path, filepath.Base(c.Path))
	if err := os.Rename(c.Path, newPath); err != nil {
		return err
	}
	c.Path = newPath
	return WriteCard(*c)
}

func CopyCard(c card.Card, destCol column.Column) (card.Card, error) {
	newCard, err := CreateCard(destCol, c.Title)
	if err != nil {
		return card.Card{}, err
	}
	newCard.Content = c.Content
	if err := WriteCard(newCard); err != nil {
		return card.Card{}, err
	}
	return newCard, nil
}

func TrashCard(c card.Card) error {
	_, err := exec.LookPath("trash")
	if err != nil {
		return fmt.Errorf("'trash' command not found, please install trash-cli")
	}
	cmd := exec.Command("trash", c.Path)
	return cmd.Run()
}

func CreateColumn(name string) (column.Column, error) {
	colPath := filepath.Join(DataDirName, name)
	if err := os.Mkdir(colPath, 0755); err != nil {
		return column.Column{}, err
	}
	return column.New(name, colPath), nil
}

func DeleteColumn(col column.Column) error {
	for _, crd := range col.Cards {
		if err := TrashCard(crd); err != nil {
			// Log or handle error, but try to continue
			fmt.Fprintf(os.Stderr, "could not trash card %s: %v\n", crd.Path, err)
		}
	}

	_, err := exec.LookPath("trash")
	if err != nil {
		return fmt.Errorf("'trash' command not found, please install trash-cli")
	}
	cmd := exec.Command("trash", col.Path)
	return cmd.Run()
}

func RenameColumn(col *column.Column, newName string) error {
	if col.Title == newName {
		return nil // No change
	}

	newPath := filepath.Join(DataDirName, newName)
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		return fmt.Errorf("column '%s' already exists", newName)
	}

	if err := os.Rename(col.Path, newPath); err != nil {
		return fmt.Errorf("could not rename column directory: %w", err)
	}

	col.Title = newName
	col.Path = newPath

	for i := range col.Cards {
		card := &col.Cards[i]
		card.Path = filepath.Join(newPath, filepath.Base(card.Path))
	}

	return nil
}

func statePath() string {
	return filepath.Join(DataDirName, StateFileName)
}

func SaveState(focusedColumn, focusedCard int, doneColumn string, showHidden bool) error {
	state := AppState{
		FocusedColumn: focusedColumn,
		FocusedCard:   focusedCard,
		DoneColumn:    doneColumn,
		ShowHidden:    showHidden,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(), data, 0644)
}

func LoadState() (AppState, error) {
	data, err := os.ReadFile(statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return AppState{}, nil
		}
		return AppState{}, err
	}

	var state AppState
	if err := json.Unmarshal(data, &state); err != nil {
		return AppState{}, err
	}
	return state, nil
}
