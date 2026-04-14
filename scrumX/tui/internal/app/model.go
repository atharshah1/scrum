package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/tui/internal/api"
	"github.com/atharshah1/scrum/scrumX/tui/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type dataLoadedMsg struct {
	board  *api.Board
	issues []api.Issue
	err    error
}

type notificationsLoadedMsg struct {
	items []api.Notification
	err   error
}

type transitionsLoadedMsg struct {
	transitions []string
	err         error
}

type transitionAppliedMsg struct{ err error }
type eventMsg struct{ event api.Event }
type eventErrMsg struct{ err error }

const eventListenTimeout = 25 * time.Second

type Model struct {
	client            *api.Client
	loading           bool
	err               error
	board             *api.Board
	issues            []api.Issue
	selectedCol       int
	selectedRow       int
	inTransitionMode  bool
	transitionOptions []string
	transitionIndex   int
	notifications     []api.Notification
	unreadCount       int
}

func NewModel() Model {
	return Model{client: api.NewClient(), loading: true}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchDataCmd(), m.fetchNotificationsCmd(), m.listenEventCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inTransitionMode {
			switch msg.String() {
			case "esc":
				m.inTransitionMode = false
				m.transitionOptions = nil
				m.transitionIndex = 0
				return m, nil
			case "up", "k":
				if m.transitionIndex > 0 {
					m.transitionIndex--
				}
				return m, nil
			case "down", "j":
				if m.transitionIndex < len(m.transitionOptions)-1 {
					m.transitionIndex++
				}
				return m, nil
			case "enter":
				selected := m.currentSelectedIssueID()
				if selected == "" || len(m.transitionOptions) == 0 {
					return m, nil
				}
				status := m.transitionOptions[m.transitionIndex]
				m.loading = true
				m.err = nil
				return m, m.applyTransitionCmd(selected, status)
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			m.loading = true
			m.err = nil
			return m, tea.Batch(m.fetchDataCmd(), m.fetchNotificationsCmd())
		case "up", "k":
			if m.selectedRow > 0 {
				m.selectedRow--
			}
		case "down", "j":
			maxRow := m.currentColumnIssueCount() - 1
			if maxRow < 0 {
				maxRow = len(m.issues) - 1
			}
			if m.selectedRow < maxRow {
				m.selectedRow++
			}
		case "left", "h":
			if m.board != nil && m.selectedCol > 0 {
				m.selectedCol--
				m.selectedRow = 0
			}
		case "right", "l":
			if m.board != nil && m.selectedCol < len(m.board.Columns)-1 {
				m.selectedCol++
				m.selectedRow = 0
			}
		case "m":
			if projectID, status := m.currentSelectedProjectAndStatus(); projectID != "" && status != "" {
				m.loading = true
				return m, m.fetchTransitionsCmd(projectID, status)
			}
		}

	case dataLoadedMsg:
		m.loading = false
		m.err = msg.err
		m.board = msg.board
		m.issues = msg.issues
		if m.selectedCol >= len(m.boardColumns()) {
			m.selectedCol = 0
		}
		if m.selectedRow >= m.currentColumnIssueCount() {
			m.selectedRow = 0
		}

	case notificationsLoadedMsg:
		m.notifications = msg.items
		m.unreadCount = 0
		for _, n := range msg.items {
			if !n.IsRead {
				m.unreadCount++
			}
		}

	case transitionsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if len(msg.transitions) == 0 {
			m.err = fmt.Errorf("no transitions available from current state")
			return m, nil
		}
		m.inTransitionMode = true
		m.transitionOptions = msg.transitions
		m.transitionIndex = 0

	case transitionAppliedMsg:
		m.loading = false
		m.inTransitionMode = false
		m.transitionOptions = nil
		m.transitionIndex = 0
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		return m, m.fetchDataCmd()

	case eventMsg:
		if msg.event.Type != "" {
			return m, tea.Batch(m.fetchDataCmd(), m.fetchNotificationsCmd(), m.listenEventCmd())
		}
		return m, m.listenEventCmd()

	case eventErrMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		return m, m.listenEventCmd()

	}
	return m, nil
}

func (m Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39")).Render("scrumX TUI")
	if m.loading {
		return fmt.Sprintf("%s\n\nLoading...\n", title)
	}
	if m.err != nil {
		return fmt.Sprintf("%s\n\nError: %v\n\nPress r to retry, q to quit.\n", title, m.err)
	}
	var b strings.Builder
	b.WriteString(title)
	b.WriteString(fmt.Sprintf("\nUnread notifications: %d\n", m.unreadCount))

	if m.inTransitionMode {
		b.WriteString("\nMove issue to:\n")
		for i, t := range m.transitionOptions {
			cursor := "  "
			if i == m.transitionIndex {
				cursor = "➜ "
			}
			b.WriteString(cursor + t + "\n")
		}
		b.WriteString("\nKeys: ↑/↓ select • Enter apply • Esc cancel\n")
		return b.String()
	}

	if m.board != nil && len(m.board.Columns) > 0 {
		b.WriteString("\nBoard View\n")
		for ci, col := range m.board.Columns {
			head := fmt.Sprintf("[%d] %s (%d)", ci+1, col.Name, len(col.Issues))
			if ci == m.selectedCol {
				head = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Render(head)
			}
			b.WriteString(head + "\n")
			for ii, issue := range col.Issues {
				cursor := "  "
				if ci == m.selectedCol && ii == m.selectedRow {
					cursor = "➜ "
				}
				b.WriteString(fmt.Sprintf("%s%s [%s|%s]\n", cursor, issue.Title, issue.Status, issue.Priority))
			}
			if len(col.Issues) == 0 {
				b.WriteString("  -\n")
			}
			b.WriteString("\n")
		}
		if issue := m.currentBoardIssue(); issue != nil {
			b.WriteString(fmt.Sprintf("Selected: %s\nID: %s\nStatus: %s\nPriority: %s\n", issue.Title, issue.ID, issue.Status, issue.Priority))
		}
	} else {
		b.WriteString("\nIssue List\n")
		for i, issue := range m.issues {
			cursor := "  "
			if i == m.selectedRow {
				cursor = "➜ "
			}
			b.WriteString(fmt.Sprintf("%s%s [%s|%s]\n", cursor, issue.Title, issue.Status, issue.Priority))
		}
		if len(m.issues) == 0 {
			b.WriteString("No issues found\n")
		}
	}
	b.WriteString("\nKeys: ←/→ switch columns • ↑/↓ move • m move issue • r refresh • q quit\n")
	return b.String()
}

func (m Model) fetchDataCmd() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return dataLoadedMsg{err: err}
		}
		if strings.TrimSpace(cfg.CurrentBoardID) != "" {
			board, err := m.client.GetBoard(cfg.CurrentBoardID, "")
			if err != nil {
				return dataLoadedMsg{err: err}
			}
			return dataLoadedMsg{board: &board}
		}
		issues, err := m.client.ListIssues()
		return dataLoadedMsg{issues: issues, err: err}
	}
}

func (m Model) fetchNotificationsCmd() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListNotifications(20)
		return notificationsLoadedMsg{items: items, err: err}
	}
}

func (m Model) fetchTransitionsCmd(projectID, currentStatus string) tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListAllowedTransitions(projectID, currentStatus)
		return transitionsLoadedMsg{transitions: items, err: err}
	}
}

func (m Model) applyTransitionCmd(issueID, status string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.UpdateIssueStatus(issueID, status)
		return transitionAppliedMsg{err: err}
	}
}

func (m Model) listenEventCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), eventListenTimeout)
		defer cancel()
		events, errs := m.client.StreamEvents(ctx)
		select {
		case evt, ok := <-events:
			if !ok {
				return eventErrMsg{err: nil}
			}
			return eventMsg{event: evt}
		case err, ok := <-errs:
			if !ok {
				return eventErrMsg{err: nil}
			}
			return eventErrMsg{err: err}
		case <-ctx.Done():
			return eventErrMsg{err: nil}
		}
	}
}

func (m Model) boardColumns() []api.BoardColumn {
	if m.board == nil {
		return nil
	}
	return m.board.Columns
}

func (m Model) currentColumnIssueCount() int {
	if m.board == nil || len(m.board.Columns) == 0 {
		return len(m.issues)
	}
	if m.selectedCol < 0 || m.selectedCol >= len(m.board.Columns) {
		return 0
	}
	return len(m.board.Columns[m.selectedCol].Issues)
}

func (m Model) currentBoardIssue() *api.BoardIssue {
	if m.board == nil || len(m.board.Columns) == 0 {
		return nil
	}
	if m.selectedCol < 0 || m.selectedCol >= len(m.board.Columns) {
		return nil
	}
	col := m.board.Columns[m.selectedCol]
	if m.selectedRow < 0 || m.selectedRow >= len(col.Issues) {
		return nil
	}
	item := col.Issues[m.selectedRow]
	return &item
}

func (m Model) currentSelectedIssueID() string {
	if issue := m.currentBoardIssue(); issue != nil {
		return issue.ID
	}
	if m.selectedRow >= 0 && m.selectedRow < len(m.issues) {
		return m.issues[m.selectedRow].ID
	}
	return ""
}

func (m Model) currentSelectedProjectAndStatus() (string, string) {
	if issue := m.currentBoardIssue(); issue != nil {
		if m.board != nil {
			return m.board.ProjectID, issue.Status
		}
		return "", issue.Status
	}
	if m.selectedRow >= 0 && m.selectedRow < len(m.issues) {
		it := m.issues[m.selectedRow]
		return it.ProjectID, it.Status
	}
	return "", ""
}
