package core

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
	"kanban/internal/models"
)

const (
	BoardFileName     = "kanban.md"
	DataDirName       = ".kanban"
	StateFileName     = "state.json"
	frontMatterSep    = "---\n"
	ArchiveColumnName = "Archived"
)

type AppState struct {
	FocusedColumn int `json:"focused_column"`
	FocusedCard   int `json:"focused_card"`
	DoneColumn    string `json:"done_column,omitempty"`
	ShowHidden    bool   `json:"show_hidden,omitempty"`
}

var cardLinkRegex = regexp.MustCompile(`\s*-\s*\[(.*?)\]\((.*?)\)`)

func LoadBoard() (models.Board, error) {
	wd, err := os.Getwd()
	if err != nil {
		return models.Board{}, err
	}
	b := models.New(wd, []models.Column{})

	f, err := os.Open(BoardFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return b, nil
		}
		return models.Board{}, err
	}
	defer f.Close()

	allCols := make([]models.Column, 0)
	var currentColumn *models.Column

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			if currentColumn != nil {
				allCols = append(allCols, *currentColumn)
			}
			title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			currentColumn = &models.Column{
				Title: title,
				Path:  filepath.Join(DataDirName, title),
				Cards: []models.Card{},
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
		return models.Board{}, err
	}

	displayCols := make([]models.Column, 0)
	var archivedCol *models.Column

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
		b.Archived = models.NewColumn(ArchiveColumnName, filepath.Join(DataDirName, ArchiveColumnName))
	}

	return b, nil
}

func WriteBoard(b models.Board) error {
	var builder strings.Builder

	allColumns := make([]models.Column, len(b.Columns))
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

func SetupMainBoard(kanbanFilePaths []string) error {
	board, err := LoadBoard()
	if err != nil {
		return err
	}

	if err := os.Mkdir(DataDirName, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	madeChanges := false
	var hasBuffer, hasProjects bool
	for _, col := range board.Columns {
		if col.Title == "buffer" {
			hasBuffer = true
		}
		if col.Title == "projects" {
			hasProjects = true
		}
	}

	if !hasBuffer {
		newCol, err := CreateColumn("buffer")
		if err != nil {
			return err
		}
		board.Columns = append(board.Columns, newCol)
		madeChanges = true
	}
	if !hasProjects {
		newCol, err := CreateColumn("projects")
		if err != nil {
			return err
		}
		board.Columns = append(board.Columns, newCol)
		madeChanges = true
	}

	var bufferCol *models.Column
	for i := range board.Columns {
		if board.Columns[i].Title == "buffer" {
			bufferCol = &board.Columns[i]
			break
		}
	}
	if bufferCol == nil {
		return fmt.Errorf("internal error: buffer column not found after creation")
	}

	existingLinks := make(map[string]struct{})
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			if card.Link != "" {
				existingLinks[card.Link] = struct{}{}
			}
		}
	}

	mainBoardDir, err := os.Getwd()
	if err != nil {
		return err
	}
	mainBoardDir, err = filepath.Abs(mainBoardDir)
	if err != nil {
		return err
	}

	newCardsAdded := false
	for _, path := range kanbanFilePaths {
		if filepath.Base(path) != BoardFileName {
			fmt.Fprintf(os.Stderr, "warning: skipping file that is not named %s: %s\n", BoardFileName, path)
			continue
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not get absolute path for %s: %v\n", path, err)
			continue
		}

		if filepath.Dir(absPath) == mainBoardDir {
			continue // Skip self
		}

		if _, exists := existingLinks[absPath]; !exists {
			cardTitle := filepath.Base(filepath.Dir(absPath))
			newCard, err := CreateCard(*bufferCol, cardTitle)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not create card for %s: %v\n", absPath, err)
				continue
			}
			newCard.Link = absPath
			if err := WriteCard(newCard); err != nil {
				fmt.Fprintf(os.Stderr, "could not update card link for %s: %v\n", absPath, err)
				continue
			}

			bufferCol.Cards = append(bufferCol.Cards, newCard)
			newCardsAdded = true
		}
	}

	if madeChanges || newCardsAdded {
		if err := WriteBoard(board); err != nil {
			return err
		}
	}

	return nil
}

func CreateSampleBoard(b *models.Board) error {
	sampleCols := []string{"Notes", "Planned", "WIP", "Done"}
	var columns []models.Column

	if err := os.Mkdir(DataDirName, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	for _, colName := range sampleCols {
		colPath := filepath.Join(DataDirName, colName)
		if err := os.Mkdir(colPath, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		col := models.NewColumn(colName, colPath)
		columns = append(columns, col)
	}
	b.Columns = columns
	return WriteBoard(*b)
}

func LoadCard(path string) (models.Card, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return models.Card{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return models.Card{}, err
	}

	parts := strings.SplitN(string(data), frontMatterSep, 3)
	if len(parts) < 3 {
		return models.Card{}, fmt.Errorf("invalid markdown format: missing front matter")
	}

	var c models.Card
	if err := yaml.Unmarshal([]byte(parts[1]), &c); err != nil {
		return models.Card{}, err
	}

	c.Content = strings.TrimSpace(parts[2])
	c.Path = path
	c.UUID = strings.TrimSuffix(filepath.Base(path), ".md")
	c.Size = fileInfo.Size()

	return c, nil
}

func CreateCard(col models.Column, title string) (models.Card, error) {
	id := uuid.New()
	now := time.Now()

	c := models.Card{
		UUID:       id.String(),
		Title:      title,
		CreatedAt:  now,
		ModifiedAt: now,
		Path:       filepath.Join(col.Path, id.String()+".md"),
	}

	if err := WriteCard(c); err != nil {
		return models.Card{}, err
	}

	fileInfo, err := os.Stat(c.Path)
	if err != nil {
		// This should not happen as we just wrote the file
		return c, nil
	}
	c.Size = fileInfo.Size()

	return c, nil
}

func WriteCard(c models.Card) error {
	c.ModifiedAt = time.Now()

	frontMatter, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}

	content := fmt.Sprintf("%s%s%s\n%s\n", frontMatterSep, string(frontMatter), frontMatterSep, c.Content)

	return os.WriteFile(c.Path, []byte(content), 0644)
}

func MoveCard(c *models.Card, destCol models.Column) error {
	newPath := filepath.Join(destCol.Path, filepath.Base(c.Path))

	if c.Path == newPath {
		return nil
	}

	err := os.Rename(c.Path, newPath)
	if err != nil {
		// This handles state desynchronization after an undo that only reverts
		// in-memory state but not filesystem operations. If the rename fails
		// because the source file doesn't exist, we check if it's because the
		// file is *already* at the destination.
		if os.IsNotExist(err) {
			if _, statErr := os.Stat(newPath); statErr == nil {
				c.Path = newPath
				return WriteCard(*c)
			}
		}
		return err
	}
	c.Path = newPath
	return WriteCard(*c)
}

func CopyCard(c models.Card, destCol models.Column) (models.Card, error) {
	newCard, err := CreateCard(destCol, c.Title)
	if err != nil {
		return models.Card{}, err
	}
	newCard.Content = c.Content
	newCard.Link = c.Link
	if err := WriteCard(newCard); err != nil {
		return models.Card{}, err
	}
	return newCard, nil
}

func TrashCard(c models.Card) error {
	_, err := exec.LookPath("trash")
	if err != nil {
		return fmt.Errorf("'trash' command not found, please install trash-cli")
	}
	cmd := exec.Command("trash", c.Path)
	return cmd.Run()
}

func CreateColumn(name string) (models.Column, error) {
	colPath := filepath.Join(DataDirName, name)
	if err := os.Mkdir(colPath, 0755); err != nil {
		return models.Column{}, err
	}
	return models.NewColumn(name, colPath), nil
}

func DeleteColumn(col models.Column) error {
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

func RenameColumn(col *models.Column, newName string) error {
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

func FlushTrash(trash []models.Card) error {
	var firstErr error
	for _, c := range trash {
		err := TrashCard(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not trash card %s: %v\n", c.Path, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// SynchronizeBoard ensures that the filesystem state matches the board state.
// It moves card files to their correct locations as defined in the board struct.
// This is useful after an undo/redo operation to ensure consistency.
func SynchronizeBoard(b models.Board) error {
	// Create a map of all card UUIDs to their correct paths from the board state.
	expectedPaths := make(map[string]string)

	allCols := make([]models.Column, 0, len(b.Columns)+1)
	allCols = append(allCols, b.Columns...)
	if b.Archived.CardCount() > 0 {
		allCols = append(allCols, b.Archived)
	}

	for _, col := range allCols {
		// Ensure column directory exists
		if err := os.MkdirAll(col.Path, 0755); err != nil {
			return fmt.Errorf("could not create directory for column '%s': %w", col.Title, err)
		}
		for _, card := range col.Cards {
			expectedPaths[card.UUID] = card.Path
		}
	}

	// Scan the .kanban directory to find the actual locations of card files.
	actualPaths := make(map[string]string)
	err := filepath.Walk(DataDirName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			cardUUID := strings.TrimSuffix(info.Name(), ".md")
			if _, err := uuid.Parse(cardUUID); err == nil {
				actualPaths[cardUUID] = path
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not scan for cards: %w", err)
	}

	// Move files that are in the wrong place.
	for cardUUID, actualPath := range actualPaths {
		if expectedPath, ok := expectedPaths[cardUUID]; ok {
			if actualPath != expectedPath {
				if err := os.Rename(actualPath, expectedPath); err != nil {
					if os.IsNotExist(err) {
						if _, statErr := os.Stat(expectedPath); statErr == nil {
							continue
						}
					}
					return fmt.Errorf("could not move card %s from %s to %s: %w", cardUUID, actualPath, expectedPath, err)
				}
			}
		}
	}
	return nil
}
