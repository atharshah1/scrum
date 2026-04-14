package notifications

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/internal/issues"
	"github.com/google/uuid"
)

// Service creates notifications in response to domain events.
type Service struct {
	repo        *Repository
	notifyActor bool
}

func NewService(repo *Repository, notifyActor bool) *Service {
	return &Service{repo: repo, notifyActor: notifyActor}
}

var mentionUUIDRegex = regexp.MustCompile(`@([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12})`)

func (s *Service) HandleEvent(event events.Event) {
	ctx := context.Background()

	switch event.Type {
	case "issue.created":
		issueID, title := issueFromPayload(event.Payload)
		if issueID == uuid.Nil {
			return
		}
		recipients := s.issueRecipients(ctx, event, issueID, nil)
		for _, userID := range recipients {
			_ = s.repo.Create(ctx, Notification{
				OrgID:    event.OrgID,
				UserID:   userID,
				Type:     "issue.created",
				Title:    title,
				Message:  "A new issue was created.",
				EntityID: issueID,
			})
		}

	case "issue.updated":
		issueID, _ := issueFromPayload(event.Payload)
		if issueID == uuid.Nil {
			return
		}
		recipients := s.issueRecipients(ctx, event, issueID, nil)
		for _, userID := range recipients {
			_ = s.repo.Create(ctx, Notification{
				OrgID:    event.OrgID,
				UserID:   userID,
				Type:     "issue.updated",
				Title:    "Issue updated",
				Message:  "An issue you are involved with was updated.",
				EntityID: issueID,
			})
		}

	case "issue.comment_created":
		issueID := uuidFromAny(event.Payload["issue_id"])
		if issueID == uuid.Nil {
			return
		}
		commentBody := commentBodyFromPayload(event.Payload["comment"])
		mentioned := s.extractMentionedUsers(ctx, event.OrgID, commentBody)
		recipients := s.issueRecipients(ctx, event, issueID, mentioned)
		for _, userID := range recipients {
			_ = s.repo.Create(ctx, Notification{
				OrgID:    event.OrgID,
				UserID:   userID,
				Type:     "issue.comment_created",
				Title:    "Comment added",
				Message:  "A new comment was added on an issue you follow.",
				EntityID: issueID,
			})
		}

	case "sprint.started":
		sprintID := uuidFromAny(event.Payload["sprint_id"])
		if sprintID == uuid.Nil {
			return
		}
		_ = s.repo.Create(ctx, Notification{
			OrgID:    event.OrgID,
			UserID:   event.ActorID,
			Type:     "sprint.started",
			Title:    "Sprint started",
			Message:  "A sprint has been started.",
			EntityID: sprintID,
		})

	case "sprint.completed":
		sprintID := uuidFromAny(event.Payload["sprint_id"])
		if sprintID == uuid.Nil {
			return
		}
		_ = s.repo.Create(ctx, Notification{
			OrgID:    event.OrgID,
			UserID:   event.ActorID,
			Type:     "sprint.completed",
			Title:    "Sprint completed",
			Message:  "A sprint has been completed.",
			EntityID: sprintID,
		})
	}
}

func issueFromPayload(payload map[string]any) (uuid.UUID, string) {
	raw := payload["issue"]
	switch issue := raw.(type) {
	case issues.Issue:
		return issue.ID, issueTitle(issue.Title)
	case *issues.Issue:
		if issue == nil {
			return uuid.Nil, "New issue created"
		}
		return issue.ID, issueTitle(issue.Title)
	case map[string]any:
		return uuidFromAny(issue["id"]), issueTitle(stringFromAny(issue["title"]))
	default:
		return uuid.Nil, "New issue created"
	}
}

func issueTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "New issue created"
	}
	return fmt.Sprintf("New issue: %s", title)
}

func (s *Service) issueRecipients(ctx context.Context, event events.Event, issueID uuid.UUID, extra map[uuid.UUID]struct{}) []uuid.UUID {
	recipients := map[uuid.UUID]struct{}{}
	watchers, err := s.repo.ListIssueWatchers(ctx, event.OrgID, issueID)
	if err == nil {
		for _, watcherID := range watchers {
			if watcherID != uuid.Nil {
				recipients[watcherID] = struct{}{}
			}
		}
	}
	if assigneeID, ok, err := s.repo.GetIssueAssignee(ctx, event.OrgID, issueID); err == nil && ok && assigneeID != uuid.Nil {
		recipients[assigneeID] = struct{}{}
	}
	for userID := range extra {
		if userID != uuid.Nil {
			recipients[userID] = struct{}{}
		}
	}
	if !s.notifyActor {
		delete(recipients, event.ActorID)
	}
	out := make([]uuid.UUID, 0, len(recipients))
	for userID := range recipients {
		out = append(out, userID)
	}
	return out
}

func (s *Service) extractMentionedUsers(ctx context.Context, orgID uuid.UUID, body string) map[uuid.UUID]struct{} {
	out := map[uuid.UUID]struct{}{}
	if strings.TrimSpace(body) == "" {
		return out
	}
	matches := mentionUUIDRegex.FindAllStringSubmatch(body, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		userID, err := uuid.Parse(m[1])
		if err != nil || userID == uuid.Nil {
			continue
		}
		ok, err := s.repo.UserExistsInOrg(ctx, orgID, userID)
		if err == nil && ok {
			out[userID] = struct{}{}
		}
	}
	return out
}

func commentBodyFromPayload(raw any) string {
	switch c := raw.(type) {
	case issues.IssueComment:
		return c.Body
	case *issues.IssueComment:
		if c == nil {
			return ""
		}
		return c.Body
	case map[string]any:
		return stringFromAny(c["body"])
	default:
		return ""
	}
}

func stringFromAny(v any) string {
	s, _ := v.(string)
	return s
}

func uuidFromAny(v any) uuid.UUID {
	switch raw := v.(type) {
	case uuid.UUID:
		return raw
	case *uuid.UUID:
		if raw == nil {
			return uuid.Nil
		}
		return *raw
	case string:
		id, _ := uuid.Parse(raw)
		return id
	default:
		return uuid.Nil
	}
}

func uuidFromPayload(m map[string]any, key string) uuid.UUID {
	id := uuidFromAny(m[key])
	return id
}
