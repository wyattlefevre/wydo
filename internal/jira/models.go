package jira

// Board represents a Jira board
type Board struct {
	ID   int
	Name string
}

// Issue represents a Jira issue with key, summary, and status
type Issue struct {
	Key     string
	Summary string
	Status  string
}
