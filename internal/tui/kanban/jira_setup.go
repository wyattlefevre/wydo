package kanban

import (
	"strings"
	"wydo/internal/config"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type jiraSetupStep int

const (
	jiraSetupStepURL jiraSetupStep = iota
	jiraSetupStepEmail
	jiraSetupStepToken
)

// JiraSetupModel is a multi-step wizard for entering Jira credentials.
// It pre-fills fields from existing config if present.
type JiraSetupModel struct {
	step      jiraSetupStep
	urlInput  textinput.Model
	emailInput textinput.Model
	tokenInput textinput.Model
	err       error
	width     int
	height    int
}

func NewJiraSetupModel(existing *config.JiraConfig) JiraSetupModel {
	url := textinput.New()
	url.Placeholder = "https://yourcompany.atlassian.net"
	url.CharLimit = 200
	url.Width = 50
	if existing != nil {
		url.SetValue(existing.BaseURL)
	}

	email := textinput.New()
	email.Placeholder = "you@yourcompany.com"
	email.CharLimit = 200
	email.Width = 50
	if existing != nil {
		email.SetValue(existing.Email)
	}

	token := textinput.New()
	token.Placeholder = "paste token here"
	token.CharLimit = 500
	token.Width = 50
	token.EchoMode = textinput.EchoPassword
	token.EchoCharacter = '•'
	if existing != nil {
		token.SetValue(existing.APIToken)
	}

	m := JiraSetupModel{
		step:       jiraSetupStepURL,
		urlInput:   url,
		emailInput: email,
		tokenInput: token,
	}
	m.urlInput.Focus()
	return m
}

func (m JiraSetupModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update returns (model, savedConfig, done).
// savedConfig is non-nil only when the user completed all steps.
func (m JiraSetupModel) Update(msg tea.KeyMsg) (JiraSetupModel, *config.JiraConfig, bool) {
	switch msg.String() {
	case "esc":
		return m, nil, true

	case "enter":
		switch m.step {
		case jiraSetupStepURL:
			if strings.TrimSpace(m.urlInput.Value()) == "" {
				return m, nil, false
			}
			m.urlInput.Blur()
			m.step = jiraSetupStepEmail
			m.emailInput.Focus()
			return m, nil, false

		case jiraSetupStepEmail:
			if strings.TrimSpace(m.emailInput.Value()) == "" {
				return m, nil, false
			}
			m.emailInput.Blur()
			m.step = jiraSetupStepToken
			m.tokenInput.Focus()
			return m, nil, false

		case jiraSetupStepToken:
			if strings.TrimSpace(m.tokenInput.Value()) == "" {
				return m, nil, false
			}
			cfg := &config.JiraConfig{
				BaseURL:  strings.TrimRight(strings.TrimSpace(m.urlInput.Value()), "/"),
				Email:    strings.TrimSpace(m.emailInput.Value()),
				APIToken: strings.TrimSpace(m.tokenInput.Value()),
			}
			return m, cfg, true
		}

	case "shift+tab", "up":
		// Allow going back a step
		switch m.step {
		case jiraSetupStepEmail:
			m.emailInput.Blur()
			m.step = jiraSetupStepURL
			m.urlInput.Focus()
		case jiraSetupStepToken:
			m.tokenInput.Blur()
			m.step = jiraSetupStepEmail
			m.emailInput.Focus()
		}
		return m, nil, false
	}

	// Forward to active input
	var cmd tea.Cmd
	switch m.step {
	case jiraSetupStepURL:
		m.urlInput, cmd = m.urlInput.Update(msg)
	case jiraSetupStepEmail:
		m.emailInput, cmd = m.emailInput.Update(msg)
	case jiraSetupStepToken:
		m.tokenInput, cmd = m.tokenInput.Update(msg)
	}
	_ = cmd
	return m, nil, false
}

func (m JiraSetupModel) View() string {
	var b strings.Builder

	b.WriteString(tagPickerTitleStyle.Render("Jira Setup"))
	b.WriteString("\n\n")

	stepLabel := func(n jiraSetupStep, label string) string {
		if m.step == n {
			return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69")).Render("▶ " + label)
		}
		if m.step > n {
			return lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("  " + label)
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("  " + label)
	}

	// Step 1: Base URL
	b.WriteString(stepLabel(jiraSetupStepURL, "Jira URL"))
	b.WriteString("\n")
	if m.step == jiraSetupStepURL {
		b.WriteString("  " + m.urlInput.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  Your Atlassian site URL, e.g. https://myco.atlassian.net"))
	} else {
		b.WriteString(pathStyle.Render("  " + m.urlInput.Value()))
	}
	b.WriteString("\n\n")

	// Step 2: Email
	b.WriteString(stepLabel(jiraSetupStepEmail, "Email"))
	b.WriteString("\n")
	if m.step == jiraSetupStepEmail {
		b.WriteString("  " + m.emailInput.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  The email address for your Atlassian account"))
	} else if m.step > jiraSetupStepEmail {
		b.WriteString(pathStyle.Render("  " + m.emailInput.Value()))
	}
	b.WriteString("\n\n")

	// Step 3: API Token
	b.WriteString(stepLabel(jiraSetupStepToken, "API Token"))
	b.WriteString("\n")
	if m.step == jiraSetupStepToken {
		b.WriteString("  " + m.tokenInput.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("  Go to id.atlassian.com → Security → API tokens → Create token"))
	}
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render("  " + m.err.Error()))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("enter: next • shift+tab: back • esc: cancel"))

	box := tagPickerBoxStyle.Width(64).Render(b.String())
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
