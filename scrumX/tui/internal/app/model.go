package app

import (
	"fmt"
	"strings"

	"github.com/atharshah1/scrum/scrumX/tui/internal/api"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type issuesLoadedMsg struct {
	issues []api.Issue
	err    error
}

type Model struct {
	client   *api.Client
	issues   []api.Issue
	selected int
	loading  bool
	err      error
}

func NewModel() Model {
	return Model{client: api.NewClient(), loading: true}
}

func (m Model) Init() tea.Cmd {
	return m.fetchIssues
}

func (m Model) fetchIssues() tea.Msg {
	issues, err := m.client.ListIssues()
	return issuesLoadedMsg{issues: issues, err: err}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			return m, m.fetchIssues
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.issues)-1 {
				m.selected++
			}
		}
	case issuesLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.issues = msg.issues
		if m.selected >= len(m.issues) {
			m.selected = 0
		}
	}
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("scrumX TUI")
	if m.loading {
		return fmt.Sprintf("%s\n\nLoading issues...\n", title)
	}
	if m.err != nil {
		return fmt.Sprintf("%s\n\nError: %v\n\nPress r to retry, q to quit.\n", title, m.err)
	}
	if len(m.issues) == 0 {
		return fmt.Sprintf("%s\n\nNo issues found.\n\nPress r to refresh, q to quit.\n", title)
	}

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\nIssues:\n")
	for i, issue := range m.issues {
		cursor := "  "
		if i == m.selected {
			cursor = "➜ "
		}
		line := fmt.Sprintf("%s%s [%s|%s]\n", cursor, issue.Title, issue.Status, issue.Priority)
		if i == m.selected {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render(line)
		}
		b.WriteString(line)
	}
	selected := m.issues[m.selected]
	b.WriteString("\nSelected:\n")
	b.WriteString(fmt.Sprintf("ID: %s\nStatus: %s\nPriority: %s\n", selected.ID, selected.Status, selected.Priority))
	b.WriteString("\nKeys: ↑/↓ move • r refresh • q quit\n")
	return b.String()
}
