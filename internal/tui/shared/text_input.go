package shared

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"wydo/internal/tui/theme"
)

var (
	textInputPromptStyle = lipgloss.NewStyle().Foreground(theme.Secondary)
	textInputErrorStyle  = theme.Error
	textInputBoxStyle    = theme.ModalBox.Padding(0, 1)
)

// TextInputModel wraps bubbles/textinput with validation and error display.
type TextInputModel struct {
	Input       textinput.Model
	Prompt      string
	Validator   func(string) error
	Placeholder string
	Error       string
	Width       int
}

// TextInputResultMsg is sent when input is confirmed or cancelled.
type TextInputResultMsg struct {
	Value     string
	Cancelled bool
}

// NewTextInput creates a new text input component.
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

// NewDateInput creates a text input configured for date entry.
func NewDateInput(prompt string) *TextInputModel {
	return NewTextInput(prompt, "yyyy-MM-dd", ValidateDateFormat)
}

// Init implements tea.Model.
func (m *TextInputModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m *TextInputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.Validator != nil {
				if err := m.Validator(m.Input.Value()); err != nil {
					m.Error = err.Error()
					return m, nil
				}
			}
			return m, func() tea.Msg {
				return TextInputResultMsg{Value: m.Input.Value()}
			}

		case "esc":
			return m, func() tea.Msg {
				return TextInputResultMsg{Cancelled: true}
			}
		}
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	m.Error = ""
	return m, cmd
}

// View implements tea.Model.
func (m *TextInputModel) View() string {
	var content string

	content += textInputPromptStyle.Render(m.Prompt+": ") + m.Input.View() + "\n"

	if m.Error != "" {
		content += textInputErrorStyle.Render("Error: "+m.Error) + "\n"
	}

	content += theme.Muted.Render("[enter] confirm  [esc] cancel")

	return textInputBoxStyle.Width(m.Width).Render(content)
}

// Value returns the current input value.
func (m *TextInputModel) Value() string {
	return m.Input.Value()
}

// SetValue sets the input value.
func (m *TextInputModel) SetValue(v string) {
	m.Input.SetValue(v)
}

// SetWidth sets both the outer box and inner input widths.
func (m *TextInputModel) SetWidth(w int) {
	m.Width = w - 4
	m.Input.Width = m.Width - lipgloss.Width(m.Prompt+": ")
}

// Focus focuses the input.
func (m *TextInputModel) Focus() tea.Cmd {
	return m.Input.Focus()
}

// ValidateDateFormat validates that the input is in yyyy-MM-dd format.
func ValidateDateFormat(s string) error {
	if s == "" {
		return nil
	}
	_, err := time.Parse("2006-01-02", s)
	if err != nil {
		return fmt.Errorf("invalid date format, use yyyy-MM-dd")
	}
	return nil
}
