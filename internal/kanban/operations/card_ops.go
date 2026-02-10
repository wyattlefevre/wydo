package operations

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"wydo/internal/kanban/fs"
	"wydo/internal/kanban/models"
)

// CreateCard creates a new card in the specified column
func CreateCard(board *models.Board, columnName string) (models.Card, error) {
	cardsDir := filepath.Join(board.Path, "cards")

	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return models.Card{}, err
	}

	defaultTitle := "New Card"
	baseFilename := ToSnakeCase(defaultTitle)
	filename := UniqueFilename(baseFilename, cardsDir, "")

	cardPath := filepath.Join(cardsDir, filename)

	card := models.Card{
		Filename: filename,
		Title:    defaultTitle,
		Tags:     []string{},
		Content:  "# New Card\n\nEnter card description here...\n",
	}

	if err := fs.WriteCard(card, cardPath); err != nil {
		return models.Card{}, err
	}

	col := board.GetColumn(columnName)
	if col != nil {
		col.Cards = append(col.Cards, card)
		if err := fs.WriteBoard(*board); err != nil {
			return models.Card{}, err
		}
	}

	return card, nil
}

// SyncCardFilename renames a card file if its title has changed
func SyncCardFilename(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	cardsDir := filepath.Join(board.Path, "cards")
	cardPath := filepath.Join(cardsDir, card.Filename)

	updatedCard, err := fs.ReadCard(cardPath)
	if err != nil {
		return err
	}

	expectedBase := ToSnakeCase(updatedCard.Title)
	expectedFilename := UniqueFilename(expectedBase, cardsDir, card.Filename)

	if expectedFilename == card.Filename {
		return nil
	}

	oldPath := cardPath
	newPath := filepath.Join(cardsDir, expectedFilename)
	if err := os.Rename(oldPath, newPath); err != nil {
		return err
	}

	card.Filename = expectedFilename

	return fs.WriteBoard(*board)
}

// EditCard opens a card in the user's editor
func EditCard(boardPath, filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	cardPath := filepath.Join(boardPath, "cards", filename)

	cmd := exec.Command(editor, cardPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// DeleteCard removes a card file and its reference from the board
func DeleteCard(board *models.Board, columnIndex, cardIndex int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := column.Cards[cardIndex]
	cardPath := filepath.Join(board.Path, "cards", card.Filename)

	if err := os.Remove(cardPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	column.Cards = append(column.Cards[:cardIndex], column.Cards[cardIndex+1:]...)

	return fs.WriteBoard(*board)
}

// MoveCard moves a card from one column to another
func MoveCard(board *models.Board, fromColIndex, cardIndex, toColIndex int) error {
	if fromColIndex < 0 || fromColIndex >= len(board.Columns) {
		return fmt.Errorf("invalid source column index")
	}
	if toColIndex < 0 || toColIndex >= len(board.Columns) {
		return fmt.Errorf("invalid destination column index")
	}

	fromCol := &board.Columns[fromColIndex]
	if cardIndex < 0 || cardIndex >= len(fromCol.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := fromCol.Cards[cardIndex]
	fromCol.Cards = append(fromCol.Cards[:cardIndex], fromCol.Cards[cardIndex+1:]...)

	toCol := &board.Columns[toColIndex]
	toCol.Cards = append(toCol.Cards, card)

	return fs.WriteBoard(*board)
}

// ReloadCard reloads a card from disk
func ReloadCard(boardPath, filename string) (models.Card, error) {
	cardPath := filepath.Join(boardPath, "cards", filename)
	return fs.ReadCard(cardPath)
}

// CollectAllTags gathers all unique tags across all cards in a board
func CollectAllTags(board *models.Board) []string {
	tagSet := make(map[string]bool)
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			for _, tag := range card.Tags {
				tagSet[tag] = true
			}
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	sortStrings(tags)
	return tags
}

func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// CollectAllProjects gathers all unique projects across all cards in a board
func CollectAllProjects(board *models.Board) []string {
	projectSet := make(map[string]bool)
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			for _, project := range card.Projects {
				projectSet[project] = true
			}
		}
	}

	projects := make([]string, 0, len(projectSet))
	for project := range projectSet {
		projects = append(projects, project)
	}

	sortStrings(projects)
	return projects
}

// UpdateCardTags updates a card's tags and persists to disk
func UpdateCardTags(board *models.Board, columnIndex, cardIndex int, tags []string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	card.Tags = tags

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	return fs.WriteCard(*card, cardPath)
}

// UpdateCardProjects updates a card's projects and persists to disk
func UpdateCardProjects(board *models.Board, columnIndex, cardIndex int, projects []string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	card.Projects = projects

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	return fs.WriteCard(*card, cardPath)
}

// UpdateCardURL updates a card's URL and persists to disk
func UpdateCardURL(board *models.Board, columnIndex, cardIndex int, url string) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	card.URL = url

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	return fs.WriteCard(*card, cardPath)
}

// UpdateCardDueDate updates a card's due date and persists to disk
func UpdateCardDueDate(board *models.Board, columnIndex, cardIndex int, dueDate *time.Time) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	card.DueDate = dueDate

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	return fs.WriteCard(*card, cardPath)
}

// UpdateCardScheduledDate updates a card's scheduled date and persists to disk
func UpdateCardScheduledDate(board *models.Board, columnIndex, cardIndex int, scheduledDate *time.Time) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	card.ScheduledDate = scheduledDate

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	return fs.WriteCard(*card, cardPath)
}

// UpdateCardPriority updates a card's priority and persists to disk
func UpdateCardPriority(board *models.Board, columnIndex, cardIndex, priority int) error {
	if columnIndex < 0 || columnIndex >= len(board.Columns) {
		return fmt.Errorf("invalid column index")
	}

	column := &board.Columns[columnIndex]
	if cardIndex < 0 || cardIndex >= len(column.Cards) {
		return fmt.Errorf("invalid card index")
	}

	card := &column.Cards[cardIndex]
	card.Priority = priority

	cardPath := filepath.Join(board.Path, "cards", card.Filename)
	return fs.WriteCard(*card, cardPath)
}

// TaskPriorityToCardPriority maps a todo.txt priority rune (A-F) to a card priority int (1-6).
// Returns 0 for no priority.
func TaskPriorityToCardPriority(p rune) int {
	if p >= 'A' && p <= 'F' {
		return int(p-'A') + 1
	}
	return 0
}

// CreateCardFromTask creates a new card in the first column of a board from task data.
func CreateCardFromTask(board *models.Board, title string, projects []string, tags []string, dueDate *time.Time, scheduledDate *time.Time, priority int) (models.Card, error) {
	if len(board.Columns) == 0 {
		return models.Card{}, fmt.Errorf("board has no columns")
	}

	cardsDir := filepath.Join(board.Path, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return models.Card{}, err
	}

	baseFilename := ToSnakeCase(title)
	filename := UniqueFilename(baseFilename, cardsDir, "")

	card := models.Card{
		Filename:      filename,
		Title:         title,
		Tags:          tags,
		Projects:      projects,
		Content:       "# " + title + "\n",
		DueDate:       dueDate,
		ScheduledDate: scheduledDate,
		Priority:      priority,
	}

	cardPath := filepath.Join(cardsDir, filename)
	if err := fs.WriteCard(card, cardPath); err != nil {
		return models.Card{}, err
	}

	board.Columns[0].Cards = append(board.Columns[0].Cards, card)
	if err := fs.WriteBoard(*board); err != nil {
		return models.Card{}, err
	}

	return card, nil
}

// OpenURL opens a URL in the default browser
func OpenURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
