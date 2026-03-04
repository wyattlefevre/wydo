package kanban

import (
	"fmt"
	"strconv"
	"strings"
	"wydo/internal/jira"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// JiraBoardPickerModel lets the user pick a Jira board to link
type JiraBoardPickerModel struct {
	boards   []jira.Board
	filtered []int
	selected int
	query    string
	input    textinput.Model
	width    int
	height   int
	err      error
}

func NewJiraBoardPickerModel(boards []jira.Board) JiraBoardPickerModel {
	ti := textinput.New()
	ti.Placeholder = "search boards or type a board ID..."
	ti.CharLimit = 60
	ti.Width = 46
	ti.Focus()

	m := JiraBoardPickerModel{
		boards: boards,
		input:  ti,
	}
	m.applyFilter()
	return m
}

func (m *JiraBoardPickerModel) applyFilter() {
	if m.query == "" {
		m.filtered = make([]int, len(m.boards))
		for i := range m.boards {
			m.filtered[i] = i
		}
	} else {
		names := make([]string, len(m.boards))
		for i, b := range m.boards {
			names[i] = b.Name
		}
		matches := fuzzy.Find(m.query, names)
		m.filtered = make([]int, len(matches))
		for i, match := range matches {
			m.filtered[i] = match.Index
		}
	}
	if m.selected >= len(m.filtered) {
		m.selected = max(0, len(m.filtered)-1)
	}
}

// boardIDFromQuery returns a non-zero board ID if the query is a plain integer.
func boardIDFromQuery(q string) int {
	if id, err := strconv.Atoi(q); err == nil && id > 0 {
		return id
	}
	return 0
}

// Update returns (model, selectedBoard, done)
func (m JiraBoardPickerModel) Update(msg tea.KeyMsg) (JiraBoardPickerModel, *jira.Board, bool) {
	switch msg.String() {
	case "esc":
		return m, nil, true
	case "enter":
		// If the query is a bare integer, use it as a board ID directly
		if id := boardIDFromQuery(m.query); id != 0 {
			return m, &jira.Board{ID: id, Name: fmt.Sprintf("Board %d", id)}, true
		}
		if len(m.filtered) > 0 && m.selected < len(m.filtered) {
			b := m.boards[m.filtered[m.selected]]
			return m, &b, true
		}
		return m, nil, true
	case "j", "down":
		if m.selected < len(m.filtered)-1 {
			m.selected++
		}
		return m, nil, false
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil, false
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	_ = cmd
	m.query = m.input.Value()
	m.applyFilter()
	return m, nil, false
}

func (m JiraBoardPickerModel) View() string {
	var lines []string
	lines = append(lines, tagPickerTitleStyle.Render("Link Jira Board"))
	lines = append(lines, "")
	lines = append(lines, "  "+m.input.View())
	lines = append(lines, "")

	if id := boardIDFromQuery(m.query); id != 0 {
		lines = append(lines, selectedListItemStyle.Render(fmt.Sprintf("► Use board ID %d directly", id)))
		lines = append(lines, helpStyle.Render("  (board name will show after linking)"))
	} else if len(m.filtered) == 0 {
		lines = append(lines, listItemStyle.Render("  No boards found"))
	} else {
		show := min(10, len(m.filtered))
		for i := 0; i < show; i++ {
			b := m.boards[m.filtered[i]]
			prefix := "  "
			style := listItemStyle
			if i == m.selected {
				prefix = "► "
				style = selectedListItemStyle
			}
			lines = append(lines, style.Render(fmt.Sprintf("%s%s (ID: %d)", prefix, b.Name, b.ID)))
		}
		if len(m.filtered) > show {
			lines = append(lines, pathStyle.Render(fmt.Sprintf("  ... %d more", len(m.filtered)-show)))
		}
	}

	if m.err != nil {
		lines = append(lines, "")
		lines = append(lines, errorStyle.Render(fmt.Sprintf("  Error: %v", m.err)))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	box := tagPickerBoxStyle.Width(64).Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

type jiraIssueInputState int

const (
	jiraIssueInputTyping   jiraIssueInputState = iota
	jiraIssueInputLoading                      // waiting for preview fetch
	jiraIssueInputPreview                      // showing fetched preview
)

// JiraIssueInputModel is a direct-ID input with live preview for linking a Jira issue.
type JiraIssueInputModel struct {
	state   jiraIssueInputState
	input   textinput.Model
	preview *jira.Issue
	err     error
	width   int
	height  int
}

func NewJiraIssueInputModel() JiraIssueInputModel {
	ti := textinput.New()
	ti.Placeholder = "e.g. CE-487"
	ti.CharLimit = 30
	ti.Width = 20
	ti.Focus()
	return JiraIssueInputModel{input: ti}
}

// SetPreview is called by board.go when the async fetch completes.
func (m *JiraIssueInputModel) SetPreview(issue *jira.Issue, err error) {
	m.state = jiraIssueInputPreview
	m.preview = issue
	m.err = err
}

// Update returns (model, keyToFetch, confirmedIssue, done).
//   - keyToFetch != "": board.go should fetch this key then call SetPreview
//   - confirmedIssue != nil: user confirmed, link this issue
//   - done && both nil: cancelled
func (m JiraIssueInputModel) Update(msg tea.KeyMsg) (JiraIssueInputModel, string, *jira.Issue, bool) {
	switch msg.String() {
	case "esc":
		if m.state == jiraIssueInputPreview {
			// Go back to typing
			m.state = jiraIssueInputTyping
			m.preview = nil
			m.err = nil
			m.input.Focus()
			return m, "", nil, false
		}
		return m, "", nil, true

	case "enter":
		switch m.state {
		case jiraIssueInputTyping:
			key := strings.TrimSpace(strings.ToUpper(m.input.Value()))
			if key == "" {
				return m, "", nil, false
			}
			m.state = jiraIssueInputLoading
			m.input.Blur()
			return m, key, nil, false
		case jiraIssueInputPreview:
			if m.preview != nil {
				return m, "", m.preview, true
			}
			// Error state — go back to edit
			m.state = jiraIssueInputTyping
			m.err = nil
			m.input.Focus()
			return m, "", nil, false
		}
	}

	if m.state == jiraIssueInputTyping {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		_ = cmd
	}
	return m, "", nil, false
}

func (m JiraIssueInputModel) View() string {
	var b strings.Builder

	b.WriteString(tagPickerTitleStyle.Render("Link Jira Issue"))
	b.WriteString("\n\n")

	switch m.state {
	case jiraIssueInputTyping:
		b.WriteString("  Issue key: " + m.input.View())
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("  enter: look up • esc: cancel"))

	case jiraIssueInputLoading:
		b.WriteString(pathStyle.Render("  Looking up " + strings.ToUpper(m.input.Value()) + "..."))

	case jiraIssueInputPreview:
		if m.err != nil {
			b.WriteString(errorStyle.Render("  " + m.err.Error()))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("  enter: try again • esc: edit key"))
		} else if m.preview != nil {
			b.WriteString(selectedListItemStyle.Render("  " + m.preview.Key))
			b.WriteString("\n")
			summary := m.preview.Summary
			if len(summary) > 54 {
				summary = summary[:51] + "..."
			}
			b.WriteString(listItemStyle.Render("  " + summary))
			b.WriteString("\n")
			b.WriteString(jiraStatusStyle.Render("  " + m.preview.Status))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("  enter: link this issue • esc: edit key"))
		}
	}

	box := tagPickerBoxStyle.Width(64).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// jiraStatusMsg is sent when background Jira status refresh completes
type jiraStatusMsg struct {
	statuses map[string]string // key -> status
}

// jiraFetchBoardsMsg carries the result of fetching Jira boards
type jiraFetchBoardsMsg struct {
	boards []jira.Board
	err    error
}

// jiraFetchSingleIssueMsg carries the result of fetching a single Jira issue
type jiraFetchSingleIssueMsg struct {
	issue *jira.Issue
	err   error
}

// fetchJiraBoards is a tea.Cmd that fetches boards from the Jira API
func fetchJiraBoards(baseURL, email, token string) tea.Cmd {
	return func() tea.Msg {
		client := jira.NewClient(baseURL, email, token)
		boards, err := client.GetBoards()
		return jiraFetchBoardsMsg{boards: boards, err: err}
	}
}

// fetchJiraSingleIssue is a tea.Cmd that fetches a single issue by key
func fetchJiraSingleIssue(baseURL, email, token, key string) tea.Cmd {
	return func() tea.Msg {
		client := jira.NewClient(baseURL, email, token)
		issue, err := client.GetIssue(key)
		return jiraFetchSingleIssueMsg{issue: issue, err: err}
	}
}

// refreshJiraStatuses is a tea.Cmd that refreshes statuses for all given issue keys
func refreshJiraStatuses(baseURL, email, token string, keys []string) tea.Cmd {
	return func() tea.Msg {
		client := jira.NewClient(baseURL, email, token)
		statuses, err := client.GetIssueStatuses(keys)
		if err != nil {
			return jiraStatusMsg{statuses: map[string]string{}}
		}
		return jiraStatusMsg{statuses: statuses}
	}
}

// jiraStatusLabel builds the display line for a card's Jira link
func jiraStatusLabel(key, status string) string {
	if status != "" {
		return key + " · " + status
	}
	return key
}
