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
	board        *api.Board
	issues       []api.Issue
	searchIssues []api.Issue
	err          error
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
type issueLoadedMsg struct {
	issue api.Issue
	err   error
}
type actionAppliedMsg struct{ err error }
type eventMsg struct{ event api.Event }
type eventErrMsg struct{ err error }
type syncStatusLoadedMsg struct {
	status api.SyncStatus
	err    error
}

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
	inIssueDetailMode bool
	issueDetail       *api.Issue
	inNotifications   bool
	notificationIndex int
	inInputMode       bool
	inputLabel        string
	inputValue        string
	inputAction       string
	inCommandMode     bool
	commandInput      string
	searchQuery       string
	syncMode          string
	syncPending       int
	syncConflicts     int
	syncDropped       int
	lastSyncedAt      time.Time
}

func NewModel() Model {
	return Model{client: api.NewClient(), loading: true}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchDataCmd(), m.fetchNotificationsCmd(), m.fetchSyncStatusCmd(), m.listenEventCmd())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.inCommandMode {
			switch msg.String() {
			case "esc":
				m.inCommandMode = false
				m.commandInput = ""
				return m, nil
			case "enter":
				command := strings.TrimSpace(m.commandInput)
				m.inCommandMode = false
				m.commandInput = ""
				switch {
				case strings.HasPrefix(command, "/search"):
					m.searchQuery = strings.TrimSpace(strings.TrimPrefix(command, "/search"))
					m.loading = true
					return m, m.fetchDataCmd()
				case strings.HasPrefix(command, ":filter"):
					m.searchQuery = strings.TrimSpace(strings.TrimPrefix(command, ":filter"))
					m.loading = true
					return m, m.fetchDataCmd()
				default:
					m.err = fmt.Errorf("unknown command: %s", command)
					return m, nil
				}
			case "backspace":
				if len(m.commandInput) > 0 {
					m.commandInput = m.commandInput[:len(m.commandInput)-1]
				}
				return m, nil
			default:
				if msg.String() == "space" || msg.String() == " " {
					m.commandInput += " "
					return m, nil
				}
				if len(msg.String()) == 1 {
					m.commandInput += msg.String()
				}
				return m, nil
			}
		}

		if m.inInputMode {
			switch msg.String() {
			case "esc":
				m.inInputMode = false
				m.inputLabel = ""
				m.inputValue = ""
				m.inputAction = ""
				return m, nil
			case "enter":
				value := strings.TrimSpace(m.inputValue)
				m.inInputMode = false
				m.inputLabel = ""
				m.inputValue = ""
				action := m.inputAction
				m.inputAction = ""
				if value == "" {
					return m, nil
				}
				issueID := m.currentSelectedIssueID()
				if issueID == "" {
					return m, nil
				}
				m.loading = true
				switch action {
				case "title":
					return m, m.updateIssueFieldCmd(issueID, api.UpdateIssueInput{Title: &value})
				case "description":
					return m, m.updateIssueFieldCmd(issueID, api.UpdateIssueInput{Description: &value})
				case "assignee":
					return m, m.updateIssueFieldCmd(issueID, api.UpdateIssueInput{AssigneeID: &value})
				case "label":
					return m, m.addLabelCmd(issueID, value)
				default:
					return m, nil
				}
			case "backspace":
				if len(m.inputValue) > 0 {
					m.inputValue = m.inputValue[:len(m.inputValue)-1]
				}
				return m, nil
			default:
				if len(msg.String()) == 1 {
					m.inputValue += msg.String()
				}
				return m, nil
			}
		}

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

		if m.inNotifications {
			switch msg.String() {
			case "esc", "n":
				m.inNotifications = false
				return m, nil
			case "up", "k":
				if m.notificationIndex > 0 {
					m.notificationIndex--
				}
				return m, nil
			case "down", "j":
				if m.notificationIndex < len(m.notifications)-1 {
					m.notificationIndex++
				}
				return m, nil
			case "enter":
				if len(m.notifications) == 0 {
					return m, nil
				}
				selected := m.notifications[m.notificationIndex]
				if selected.IsRead {
					return m, nil
				}
				m.loading = true
				return m, m.markNotificationReadCmd(selected.ID)
			case "A":
				m.loading = true
				return m, m.markAllNotificationsReadCmd()
			}
		}

		if m.inIssueDetailMode {
			switch msg.String() {
			case "esc", "d":
				m.inIssueDetailMode = false
				return m, nil
			case "t":
				m.inInputMode = true
				m.inputLabel = "New title"
				m.inputAction = "title"
				m.inputValue = ""
				return m, nil
			case "e":
				m.inInputMode = true
				m.inputLabel = "New description"
				m.inputAction = "description"
				m.inputValue = ""
				return m, nil
			case "a":
				m.inInputMode = true
				m.inputLabel = "Assignee ID"
				m.inputAction = "assignee"
				m.inputValue = ""
				return m, nil
			case "l":
				m.inInputMode = true
				m.inputLabel = "Label to add"
				m.inputAction = "label"
				m.inputValue = ""
				return m, nil
			case "p":
				if m.issueDetail == nil {
					return m, nil
				}
				next := nextPriority(m.issueDetail.Priority)
				issueID := m.currentSelectedIssueID()
				m.loading = true
				return m, m.updateIssueFieldCmd(issueID, api.UpdateIssueInput{Priority: &next})
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/", ":":
			m.inCommandMode = true
			m.commandInput = msg.String()
			return m, nil
		case "r":
			m.loading = true
			m.err = nil
			return m, tea.Batch(m.fetchDataCmd(), m.fetchNotificationsCmd(), m.fetchSyncStatusCmd())
		case "S":
			m.loading = true
			return m, tea.Batch(m.syncNowCmd(), m.fetchSyncStatusCmd())
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
		case "d":
			id := m.currentSelectedIssueID()
			if id == "" {
				return m, nil
			}
			m.loading = true
			m.inIssueDetailMode = true
			return m, m.fetchIssueCmd(id)
		case "n":
			m.inNotifications = !m.inNotifications
			return m, nil
		}

	case dataLoadedMsg:
		m.loading = false
		m.err = msg.err
		if msg.board != nil {
			if strings.TrimSpace(m.searchQuery) != "" {
				m.board = filterBoardByIssues(msg.board, msg.searchIssues)
			} else {
				m.board = msg.board
			}
		} else {
			m.board = nil
		}
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

	case syncStatusLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.syncMode = msg.status.Mode
		m.syncPending = msg.status.PendingOps
		m.syncConflicts = msg.status.PendingConflicts
		m.syncDropped = msg.status.DroppedOps
		m.lastSyncedAt = msg.status.LastSyncedAt

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
		return m, tea.Batch(m.fetchDataCmd(), m.fetchSyncStatusCmd())

	case issueLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.issueDetail = &msg.issue
		m.err = nil
		return m, nil

	case actionAppliedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		cmds := []tea.Cmd{m.fetchDataCmd(), m.fetchNotificationsCmd(), m.fetchSyncStatusCmd()}
		if m.inIssueDetailMode {
			if id := m.currentSelectedIssueID(); id != "" {
				cmds = append(cmds, m.fetchIssueCmd(id))
			}
		}
		return m, tea.Batch(cmds...)

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
	lastSyncText := "-"
	if !m.lastSyncedAt.IsZero() {
		lastSyncText = m.lastSyncedAt.Local().Format(time.RFC3339)
	}
	if strings.TrimSpace(m.syncMode) == "" {
		m.syncMode = "online"
	}
	b.WriteString(fmt.Sprintf("Mode: %s | Pending sync ops: %d | Conflicts: %d | Dropped ops: %d | Last sync: %s\n", m.syncMode, m.syncPending, m.syncConflicts, m.syncDropped, lastSyncText))
	if strings.TrimSpace(m.searchQuery) != "" {
		b.WriteString(fmt.Sprintf("Active filter: %s\n", m.searchQuery))
	}

	if m.inCommandMode {
		b.WriteString(fmt.Sprintf("\nCommand: %s\n", m.commandInput))
		b.WriteString("Use /search <query> or :filter <query> • Enter apply • Esc cancel\n")
		return b.String()
	}

	if m.inInputMode {
		b.WriteString(fmt.Sprintf("\n%s: %s\n", m.inputLabel, m.inputValue))
		b.WriteString("Keys: type • Enter apply • Esc cancel\n")
		return b.String()
	}

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

	if m.inNotifications {
		b.WriteString("\nNotifications\n")
		if len(m.notifications) == 0 {
			b.WriteString("No notifications\n")
		}
		for i, n := range m.notifications {
			cursor := "  "
			if i == m.notificationIndex {
				cursor = "➜ "
			}
			state := "unread"
			if n.IsRead {
				state = "read"
			}
			b.WriteString(fmt.Sprintf("%s[%s] %s (%s)\n", cursor, state, n.Title, n.Type))
		}
		b.WriteString("\nKeys: ↑/↓ select • Enter mark read • A mark all • Esc close\n")
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
	if m.inIssueDetailMode && m.issueDetail != nil {
		assignee := "-"
		if m.issueDetail.AssigneeID != nil && strings.TrimSpace(*m.issueDetail.AssigneeID) != "" {
			assignee = *m.issueDetail.AssigneeID
		}
		sprint := "-"
		if m.issueDetail.SprintID != nil && strings.TrimSpace(*m.issueDetail.SprintID) != "" {
			sprint = *m.issueDetail.SprintID
		}
		b.WriteString("\nIssue Detail\n")
		b.WriteString(fmt.Sprintf("Title: %s\n", m.issueDetail.Title))
		b.WriteString(fmt.Sprintf("Status: %s\nPriority: %s\nType: %s\n", m.issueDetail.Status, m.issueDetail.Priority, m.issueDetail.IssueType))
		b.WriteString(fmt.Sprintf("Assignee: %s\nSprint: %s\n", assignee, sprint))
		if len(m.issueDetail.Labels) > 0 {
			b.WriteString(fmt.Sprintf("Labels: %s\n", strings.Join(m.issueDetail.Labels, ", ")))
		}
		if strings.TrimSpace(m.issueDetail.Description) != "" {
			b.WriteString(fmt.Sprintf("Description: %s\n", m.issueDetail.Description))
		}
		b.WriteString("Keys: t title • e description • a assignee • l add label • p cycle priority • Esc close\n")
		return b.String()
	}
	b.WriteString("\nKeys: ←/→ switch columns • ↑/↓ move • m move issue • d details/edit • /search or :filter • n notifications • r refresh • S sync now • q quit\n")
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
			if strings.TrimSpace(m.searchQuery) == "" {
				return dataLoadedMsg{board: &board}
			}
			items, err := m.client.SearchIssuesSmart(m.searchQuery)
			return dataLoadedMsg{board: &board, searchIssues: items, err: err}
		}
		if strings.TrimSpace(m.searchQuery) != "" {
			issues, err := m.client.SearchIssuesSmart(m.searchQuery)
			return dataLoadedMsg{issues: issues, err: err}
		}
		issues, err := m.client.ListIssuesSmart()
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
		err := m.client.UpdateIssueStatusSmart(issueID, status)
		return transitionAppliedMsg{err: err}
	}
}

func (m Model) fetchIssueCmd(issueID string) tea.Cmd {
	return func() tea.Msg {
		item, err := m.client.GetIssueSmart(issueID)
		return issueLoadedMsg{issue: item, err: err}
	}
}

func (m Model) updateIssueFieldCmd(issueID string, input api.UpdateIssueInput) tea.Cmd {
	return func() tea.Msg {
		err := m.client.UpdateIssueSmart(issueID, input)
		return actionAppliedMsg{err: err}
	}
}

func (m Model) addLabelCmd(issueID, label string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.AddIssueLabelSmart(issueID, label)
		return actionAppliedMsg{err: err}
	}
}

func (m Model) fetchSyncStatusCmd() tea.Cmd {
	return func() tea.Msg {
		status, err := m.client.SyncStatus()
		return syncStatusLoadedMsg{status: status, err: err}
	}
}

func (m Model) syncNowCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.client.SyncNow()
		return actionAppliedMsg{err: err}
	}
}

func (m Model) markNotificationReadCmd(notificationID string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.MarkNotificationRead(notificationID)
		return actionAppliedMsg{err: err}
	}
}

func (m Model) markAllNotificationsReadCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.client.MarkAllNotificationsRead()
		return actionAppliedMsg{err: err}
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

func filterBoardByIssues(board *api.Board, matches []api.Issue) *api.Board {
	if board == nil {
		return nil
	}
	if len(matches) == 0 {
		if len(board.Columns) == 0 {
			return board
		}
		cloned := *board
		cloned.Columns = make([]api.BoardColumn, 0, len(board.Columns))
		for _, column := range board.Columns {
			next := column
			next.Issues = []api.BoardIssue{}
			cloned.Columns = append(cloned.Columns, next)
		}
		return &cloned
	}
	allowed := make(map[string]struct{}, len(matches))
	for _, item := range matches {
		allowed[item.ID] = struct{}{}
	}
	cloned := *board
	cloned.Columns = make([]api.BoardColumn, 0, len(board.Columns))
	for _, column := range board.Columns {
		next := column
		filtered := make([]api.BoardIssue, 0, len(column.Issues))
		for _, issue := range column.Issues {
			if _, ok := allowed[issue.ID]; ok {
				filtered = append(filtered, issue)
			}
		}
		next.Issues = filtered
		cloned.Columns = append(cloned.Columns, next)
	}
	return &cloned
}

func nextPriority(current string) string {
	switch strings.ToLower(strings.TrimSpace(current)) {
	case "low":
		return "medium"
	case "medium":
		return "high"
	case "high":
		return "critical"
	default:
		return "low"
	}
}
