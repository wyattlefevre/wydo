package data

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
)

type Priority rune

const (
	PriorityA    Priority = 'A'
	PriorityB    Priority = 'B'
	PriorityC    Priority = 'C'
	PriorityD    Priority = 'D'
	PriorityE    Priority = 'E'
	PriorityF    Priority = 'F'
	PriorityNone Priority = 0
)

type Task struct {
	ID             string
	Name           string
	Projects       []string
	Contexts       []string
	Done           bool
	Tags           map[string]string
	CreatedDate    string
	CompletionDate string
	Priority       Priority
	File           string
}

func (t *Task) HasProject(project string) bool {
	return slices.Contains(t.Projects, project)
}

func (t *Task) AddProject(project string) {
	if !t.HasProject(project) {
		t.Projects = append(t.Projects, project)
	}
}

func (t *Task) RemoveProject(project string) {
	for i, p := range t.Projects {
		if p == project {
			t.Projects = append(t.Projects[:i], t.Projects[i+1:]...)
			break
		}
	}
}

func (t *Task) HasContext(context string) bool {
	return slices.Contains(t.Contexts, context)
}

func (t *Task) AddContext(context string) {
	if !t.HasContext(context) {
		t.Contexts = append(t.Contexts, context)
	}
}

func (t *Task) RemoveContext(context string) {
	for i, c := range t.Contexts {
		if c == context {
			t.Contexts = append(t.Contexts[:i], t.Contexts[i+1:]...)
			break
		}
	}
}

func (t *Task) GetDueDate() string {
	return t.Tags["due"]
}

func (t *Task) SetDueDate(date string) {
	if t.Tags == nil {
		t.Tags = make(map[string]string)
	}
	t.Tags["due"] = date
}

func (t *Task) GetScheduledDate() string {
	return t.Tags["scheduled"]
}

func (t *Task) SetScheduledDate(date string) {
	if t.Tags == nil {
		t.Tags = make(map[string]string)
	}
	t.Tags["scheduled"] = date
}

func (t Task) String() string {
	var parts []string

	// Done status
	if t.Done {
		parts = append(parts, "x")
	}

	// For non-completed tasks: priority comes before dates
	if !t.Done && t.Priority != 0 {
		parts = append(parts, "("+string(t.Priority)+")")
	}

	// Dates
	if t.CompletionDate != "" {
		parts = append(parts, t.CompletionDate)
	}

	if t.CreatedDate != "" {
		parts = append(parts, t.CreatedDate)
	}

	// For completed tasks: priority comes after dates
	if t.Done && t.Priority != 0 {
		parts = append(parts, "("+string(t.Priority)+")")
	}

	// Name
	if t.Name != "" {
		parts = append(parts, t.Name)
	}

	// Projects
	for _, p := range t.Projects {
		parts = append(parts, "+"+p)
	}

	// Contexts
	for _, c := range t.Contexts {
		parts = append(parts, "@"+c)
	}

	// Tags — sorted for deterministic output
	tagKeys := make([]string, 0, len(t.Tags))
	for k := range t.Tags {
		tagKeys = append(tagKeys, k)
	}
	sort.Strings(tagKeys)
	for _, k := range tagKeys {
		parts = append(parts, k+":"+t.Tags[k])
	}

	return strings.Join(parts, " ")
}

func (t Task) Print() {
	fmt.Printf("ID: %s\n", t.ID)
	fmt.Printf("Name: %s\n", t.Name)
	fmt.Printf("Projects: %v\n", t.Projects)
	fmt.Printf("Contexts: %v\n", t.Contexts)
	fmt.Printf("Done: %v\n", t.Done)
	fmt.Printf("Tags: %v\n", t.Tags)
	fmt.Printf("CreatedDate: %s\n", t.CreatedDate)
	fmt.Printf("CompletionDate: %s\n", t.CompletionDate)
	fmt.Printf("Priority: %c\n", t.Priority)
}

func ParseTask(input string, id string, file string) Task {
	input = strings.TrimSpace(input)
	input = CollapseWhitespace(input)

	var t Task
	t.ID = id
	t.File = file

	if len(input) == 0 {
		return t
	}

	// Check for completion marker
	if strings.HasPrefix(input, "x ") {
		t.Done = true
		input = input[2:]
	}

	// Check for priority (can appear here for both completed and non-completed tasks)
	t.Priority = ParsePriority(input)
	if t.Priority != PriorityNone {
		input = input[4:] // "(A) " = 4 chars
	}

	input = strings.TrimLeft(input, " ")

	// Parse first date
	firstDate := ""
	if len(input) >= 10 {
		firstDate = ParseDate(input[:10])
		if firstDate != "" {
			input = input[10:]
			input = strings.TrimLeft(input, " ")
		}
	}

	// Parse second date (only if first date was found)
	secondDate := ""
	if firstDate != "" && len(input) >= 10 {
		secondDate = ParseDate(input[:10])
		if secondDate != "" {
			input = input[10:]
			input = strings.TrimLeft(input, " ")
		}
	}

	// For completed tasks, priority might come after dates (alternative format)
	if t.Done && t.Priority == PriorityNone {
		t.Priority = ParsePriority(input)
		if t.Priority != PriorityNone {
			input = input[4:] // "(A) " = 4 chars
		}
	}

	// Assign dates based on completion status
	if !t.Done && firstDate != "" {
		t.CreatedDate = firstDate
	}
	if t.Done {
		if firstDate != "" && secondDate != "" {
			t.CompletionDate = firstDate
			t.CreatedDate = secondDate
		} else if firstDate != "" {
			t.CompletionDate = firstDate
		}
	}

	input = strings.TrimLeft(input, " ")

	if len(input) == 0 {
		return t
	}

	// Find first metadata marker
	firstMetaIdx := FirstMetaIndex(
		FirstProjectIndex(input),
		FirstContextIndex(input),
		FirstTagIndex(input),
	)

	if firstMetaIdx < 0 {
		t.Name = strings.TrimSpace(input)
		return t
	}

	t.Name = strings.TrimSpace(input[:firstMetaIdx])

	t.Projects = ParseProjects(input)
	sort.Strings(t.Projects)

	t.Contexts = ParseContexts(input)
	sort.Strings(t.Contexts)

	t.Tags = ParseTags(input)

	return t
}

func CollapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func FirstProjectIndex(s string) int {
	re := regexp.MustCompile(`[ \t]\+[A-Za-z0-9]`)
	loc := re.FindStringIndex(s)
	if loc != nil {
		return loc[0] + 1
	}
	return -1
}

func FirstContextIndex(s string) int {
	re := regexp.MustCompile(`[ \t]\@[A-Za-z0-9]`)
	loc := re.FindStringIndex(s)
	if loc != nil {
		return loc[0] + 1
	}
	return -1
}

func FirstTagIndex(s string) int {
	re := regexp.MustCompile(`[ \t][A-Za-z0-9]+\:[A-Za-z0-9]+`)
	loc := re.FindStringIndex(s)
	if loc != nil {
		return loc[0] + 1
	}
	return -1
}

func ParseProjects(s string) []string {
	re := regexp.MustCompile(`[ \t]\+[A-Za-z0-9][A-Za-z0-9_-]*`)
	matches := re.FindAllString(s, -1)
	for i, m := range matches {
		matches[i] = m[2:]
	}
	return matches
}

func ParseContexts(s string) []string {
	re := regexp.MustCompile(`[ \t]\@[A-Za-z0-9]+`)
	matches := re.FindAllString(s, -1)
	for i, m := range matches {
		matches[i] = m[2:]
	}
	return matches
}

func ParseTags(s string) map[string]string {
	re := regexp.MustCompile(`[ \t]([A-Za-z0-9]+)\:([A-Za-z0-9-]+)`)
	matches := re.FindAllStringSubmatch(s, -1)
	tags := make(map[string]string)
	for _, m := range matches {
		if len(m) == 3 {
			key := m[1]
			value := m[2]
			tags[key] = value
		}
	}
	return tags
}

func ParsePriority(s string) Priority {
	re := regexp.MustCompile(`^\(([A-Fa-f])\)`)
	matches := re.FindStringSubmatch(s)
	if matches != nil {
		switch strings.ToUpper(matches[1]) {
		case "A":
			return PriorityA
		case "B":
			return PriorityB
		case "C":
			return PriorityC
		case "D":
			return PriorityD
		case "E":
			return PriorityE
		case "F":
			return PriorityF
		}
	}
	return PriorityNone
}

func FirstMetaIndex(i1 int, i2 int, i3 int) int {
	min := -1
	for _, v := range []int{i1, i2, i3} {
		if v >= 0 && (min == -1 || v < min) {
			min = v
		}
	}
	return min
}

func ParseDate(s string) string {
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return s
	}
	return ""
}
