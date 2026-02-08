package models

// Column represents a kanban column
type Column struct {
	Name  string
	Cards []Card
}
