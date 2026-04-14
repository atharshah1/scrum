package notifications

import (
	"context"
	"fmt"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/google/uuid"
)

// Service creates notifications in response to domain events.
type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// HandleEvent creates in-app notifications for events that involve watchers or
// direct mention of a user. Currently creates a notification for the actor
// themselves as a proof-of-concept; a full implementation would fan out to all
// relevant users (watchers, assignee, etc.) based on event payload.
func (s *Service) HandleEvent(event events.Event) {
	ctx := context.Background()

	switch event.Type {
	case "issue.created":
		issue, _ := event.Payload["issue"].(map[string]any)
		if issue == nil {
			return
		}
		issueID := uuidFromPayload(issue, "id")
		title := fmt.Sprintf("New issue created")
		if t, ok := issue["title"].(string); ok && t != "" {
			title = fmt.Sprintf("New issue: %s", t)
		}
		_ = s.repo.Create(ctx, Notification{
			OrgID:    event.OrgID,
			UserID:   event.ActorID,
			Type:     "issue.created",
			Title:    title,
			Message:  "You created a new issue.",
			EntityID: issueID,
		})

	case "issue.updated":
		issue, _ := event.Payload["issue"].(map[string]any)
		if issue == nil {
			return
		}
		issueID := uuidFromPayload(issue, "id")
		_ = s.repo.Create(ctx, Notification{
			OrgID:    event.OrgID,
			UserID:   event.ActorID,
			Type:     "issue.updated",
			Title:    "Issue updated",
			Message:  "An issue you worked on was updated.",
			EntityID: issueID,
		})

	case "issue.comment_created":
		issueIDRaw, _ := event.Payload["issue_id"].(string)
		issueID, _ := uuid.Parse(issueIDRaw)
		_ = s.repo.Create(ctx, Notification{
			OrgID:    event.OrgID,
			UserID:   event.ActorID,
			Type:     "issue.comment_created",
			Title:    "Comment added",
			Message:  "A comment was added to an issue.",
			EntityID: issueID,
		})

	case "sprint.started":
		sprintIDRaw, _ := event.Payload["sprint_id"].(string)
		sprintID, _ := uuid.Parse(sprintIDRaw)
		_ = s.repo.Create(ctx, Notification{
			OrgID:    event.OrgID,
			UserID:   event.ActorID,
			Type:     "sprint.started",
			Title:    "Sprint started",
			Message:  "A sprint has been started.",
			EntityID: sprintID,
		})

	case "sprint.completed":
		sprintIDRaw, _ := event.Payload["sprint_id"].(string)
		sprintID, _ := uuid.Parse(sprintIDRaw)
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

func uuidFromPayload(m map[string]any, key string) uuid.UUID {
	raw, _ := m[key].(string)
	id, _ := uuid.Parse(raw)
	return id
}
