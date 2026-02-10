package tasks

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	inputPromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	inputErrorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	inputBoxStyle    = lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("4")).Padding(0, 1)
)

// TextInputModel wraps bubbles/textinput with validation
type TextInputModel struct {
	Input       textinput.Model
	Prompt      string
	Validator   func(string) error
	Placeholder string
	Error       string
	Width       int
}

// TextInputResultMsg is sent when input is confirmed or cancelled
type TextInputResultMsg struct {
	Value     string
	Cancelled bool
}

// NewTextInput creates a new text input component
func NewTextInput(prompt string, placeholder string, validator func(string) error) *TextInputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.CharLimit = 256
	return &TextInputModel{
		Input:       ti,
		Prompt:      prompt,
		Placeholder: placeholder,
		Validator:   validator,
	}
}

// NewDateInput creates a text input configured for date entry
func NewDateInput(prompt string) *TextInputModel {
	return NewTextInput(prompt, "yyyy-MM-dd", ValidateDateFormat)
}

// Init implements tea.Model
func (m *TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model
func (m *TextInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// Validate before accepting
			if m.Validator != nil {
				if err := m.Validator(m.Input.Value()); err != nil {
					m.Error = err.Error()
					return m, nil
				}
			}
			return m, func() tea.Msg {
				return TextInputResultMsg{
					Value:     m.Input.Value(),
					Cancelled: false,
				}
			}

		case "esc":
			return m, func() tea.Msg {
				return TextInputResultMsg{
					Value:     "",
					Cancelled: true,
				}
			}
		}
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)

	// Clear error when user types
	m.Error = ""

	return m, cmd
}

// View implements tea.Model
func (m *TextInputModel) View() string {
	var content string

	content += inputPromptStyle.Render(m.Prompt+": ") + m.Input.View() + "\n"

	if m.Error != "" {
		content += inputErrorStyle.Render("Error: " + m.Error) + "\n"
	}

	content += lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("[enter] confirm  [esc] cancel")

	return inputBoxStyle.Width(m.Width).Render(content)
}

// Value returns the current input value
func (m *TextInputModel) Value() string {
	return m.Input.Value()
}

// SetValue sets the input value
func (m *TextInputModel) SetValue(v string) {
	m.Input.SetValue(v)
}

// SetWidth sets both the outer box and inner input widths
func (m *TextInputModel) SetWidth(w int) {
	// Account for border (2) and padding (2)
	m.Width = w - 4
	// Inner input accounts for prompt text
	m.Input.Width = m.Width - lipgloss.Width(m.Prompt+": ")
}

// Focus focuses the input
func (m *TextInputModel) Focus() tea.Cmd {
	return m.Input.Focus()
}

// ValidateDateFormat validates that the input is in yyyy-MM-dd format
func ValidateDateFormat(s string) error {
	if s == "" {
		return nil // Allow empty
	}
	_, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("invalid date format, use yyyy-MM-dd")
	}
	return nil
}
