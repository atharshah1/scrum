package issues

import (
	"context"
	"errors"
	"strings"

	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/google/uuid"
)

type Service struct {
	repo *Repository
	bus  events.Publisher
}

func NewService(repo *Repository, bus events.Publisher) *Service {
	return &Service{repo: repo, bus: bus}
}

func (s *Service) Create(ctx context.Context, orgID, actorID uuid.UUID, input CreateIssueInput) (Issue, error) {
	if !ValidIssueType(strings.ToLower(input.IssueType)) && input.IssueType != "" {
		return Issue{}, errors.New("invalid issue_type")
	}
	ok, err := s.repo.ProjectExists(ctx, orgID, input.ProjectID)
	if err != nil {
		return Issue{}, err
	}
	if !ok {
		return Issue{}, errors.New("project not found for org")
	}
	if input.ParentID != uuid.Nil {
		ok, err = s.repo.IssueBelongsToProject(ctx, orgID, input.ParentID, input.ProjectID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("parent issue must belong to same project")
		}
		parentIssue, err := s.repo.GetByID(ctx, orgID, input.ParentID)
		if err != nil {
			return Issue{}, err
		}
		if err := validateHierarchy(strings.ToLower(input.IssueType), parentIssue.IssueType); err != nil {
			return Issue{}, err
		}
	} else if strings.EqualFold(input.IssueType, IssueTypeStory) || strings.EqualFold(input.IssueType, IssueTypeTask) || strings.EqualFold(input.IssueType, IssueTypeBug) {
		// stories/tasks/bugs can still be top-level in backlog if desired, so allow nil parent
	}
	if input.SprintID != uuid.Nil {
		ok, err = s.repo.SprintBelongsToProject(ctx, orgID, input.SprintID, input.ProjectID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("sprint must belong to same project")
		}
	}
	issue := NewIssue(orgID, input.ProjectID, actorID, input)
	created, err := s.repo.Create(ctx, issue)
	if err != nil {
		return Issue{}, err
	}
	_ = s.repo.AddActivity(ctx, orgID, created.ID, actorID, "created", "", "", "")
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.created", actorID, map[string]any{"issue": created}))
	return created, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter ListIssuesFilter) ([]Issue, int, error) {
	return s.repo.List(ctx, orgID, filter)
}

func (s *Service) Get(ctx context.Context, orgID, issueID uuid.UUID) (Issue, error) {
	return s.repo.GetByID(ctx, orgID, issueID)
}

func (s *Service) Update(ctx context.Context, orgID, actorID, issueID uuid.UUID, input UpdateIssueInput) (Issue, error) {
	current, err := s.repo.GetByID(ctx, orgID, issueID)
	if err != nil {
		return Issue{}, err
	}
	if input.IssueType != nil && !ValidIssueType(strings.ToLower(*input.IssueType)) {
		return Issue{}, errors.New("invalid issue_type")
	}
	if input.ParentID != nil && *input.ParentID != uuid.Nil {
		ok, err := s.repo.IssueBelongsToProject(ctx, orgID, *input.ParentID, current.ProjectID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("parent issue must belong to same project")
		}
		parentIssue, err := s.repo.GetByID(ctx, orgID, *input.ParentID)
		if err != nil {
			return Issue{}, err
		}
		targetType := current.IssueType
		if input.IssueType != nil {
			targetType = *input.IssueType
		}
		if err := validateHierarchy(strings.ToLower(targetType), parentIssue.IssueType); err != nil {
			return Issue{}, err
		}
	}
	if input.SprintID != nil && *input.SprintID != uuid.Nil {
		ok, err := s.repo.SprintBelongsToProject(ctx, orgID, *input.SprintID, current.ProjectID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("sprint must belong to same project")
		}
	}
	if input.Status != nil {
		valid, err := s.repo.IsValidTransition(ctx, orgID, current.ProjectID, current.Status, *input.Status)
		if err != nil {
			return Issue{}, err
		}
		if !valid {
			return Issue{}, errors.New("invalid status transition")
		}
	}
	issue, err := s.repo.Update(ctx, orgID, issueID, input)
	if err != nil {
		return Issue{}, err
	}
	_ = s.repo.AddActivity(ctx, orgID, issue.ID, actorID, "updated", "", "", "")
	if input.Status != nil && *input.Status != current.Status {
		_ = s.repo.AddActivity(ctx, orgID, issue.ID, actorID, "status_changed", "status", current.Status, *input.Status)
	}
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.updated", actorID, map[string]any{"issue": issue}))
	return issue, nil
}

func (s *Service) Delete(ctx context.Context, orgID, actorID, issueID uuid.UUID) error {
	if err := s.repo.Delete(ctx, orgID, issueID); err != nil {
		return err
	}
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.deleted", actorID, map[string]any{"issue_id": issueID}))
	return nil
}

func (s *Service) AddRelation(ctx context.Context, orgID, actorID, issueID, relatedIssueID uuid.UUID, relationType string) error {
	if relationType == "" {
		return errors.New("relation_type is required")
	}
	if _, err := s.repo.GetByID(ctx, orgID, issueID); err != nil {
		return err
	}
	if _, err := s.repo.GetByID(ctx, orgID, relatedIssueID); err != nil {
		return err
	}
	if err := s.repo.AddRelation(ctx, orgID, issueID, relatedIssueID, relationType); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "relation_added", "relation_type", "", relationType)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.relation_added", actorID, map[string]any{"issue_id": issueID, "related_issue_id": relatedIssueID, "relation_type": relationType}))
	return nil
}

func (s *Service) ListRelations(ctx context.Context, orgID, issueID uuid.UUID) ([]map[string]any, error) {
	return s.repo.ListRelations(ctx, orgID, issueID)
}

func (s *Service) AddWatcher(ctx context.Context, orgID, actorID, issueID, userID uuid.UUID) error {
	if err := s.repo.AddWatcher(ctx, orgID, issueID, userID); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "watcher_added", "user_id", "", userID.String())
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.watcher_added", actorID, map[string]any{"issue_id": issueID, "user_id": userID}))
	return nil
}

func (s *Service) RemoveWatcher(ctx context.Context, orgID, actorID, issueID, userID uuid.UUID) error {
	if err := s.repo.RemoveWatcher(ctx, orgID, issueID, userID); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "watcher_removed", "user_id", userID.String(), "")
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.watcher_removed", actorID, map[string]any{"issue_id": issueID, "user_id": userID}))
	return nil
}

func (s *Service) ListWatchers(ctx context.Context, orgID, issueID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.ListWatchers(ctx, orgID, issueID)
}

func (s *Service) AddLabel(ctx context.Context, orgID, actorID, issueID uuid.UUID, label string) error {
	if strings.TrimSpace(label) == "" {
		return errors.New("label is required")
	}
	if err := s.repo.AddLabel(ctx, orgID, issueID, label); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "label_added", "label", "", label)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.label_added", actorID, map[string]any{"issue_id": issueID, "label": label}))
	return nil
}

func (s *Service) RemoveLabel(ctx context.Context, orgID, actorID, issueID uuid.UUID, label string) error {
	if strings.TrimSpace(label) == "" {
		return errors.New("label is required")
	}
	if err := s.repo.RemoveLabel(ctx, orgID, issueID, label); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "label_removed", "label", label, "")
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.label_removed", actorID, map[string]any{"issue_id": issueID, "label": label}))
	return nil
}

func (s *Service) ListLabels(ctx context.Context, orgID, issueID uuid.UUID) ([]string, error) {
	return s.repo.ListLabels(ctx, orgID, issueID)
}

func (s *Service) CreateComment(ctx context.Context, orgID, actorID, issueID uuid.UUID, body string) (IssueComment, error) {
	if strings.TrimSpace(body) == "" {
		return IssueComment{}, errors.New("body is required")
	}
	comment, err := s.repo.CreateComment(ctx, orgID, issueID, actorID, body)
	if err != nil {
		return IssueComment{}, err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "comment_created", "comment", "", body)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.comment_created", actorID, map[string]any{"issue_id": issueID, "comment": comment}))
	return comment, nil
}

func (s *Service) ListComments(ctx context.Context, orgID, issueID uuid.UUID) ([]IssueComment, error) {
	return s.repo.ListComments(ctx, orgID, issueID)
}

func (s *Service) ListActivities(ctx context.Context, orgID, issueID uuid.UUID) ([]IssueActivity, error) {
	return s.repo.ListActivities(ctx, orgID, issueID)
}

func validateHierarchy(childType, parentType string) error {
	childType = strings.ToLower(childType)
	parentType = strings.ToLower(parentType)
	switch childType {
	case IssueTypeEpic:
		return errors.New("epic cannot have parent")
	case IssueTypeStory:
		if parentType != IssueTypeEpic {
			return errors.New("story parent must be epic")
		}
	case IssueTypeTask, IssueTypeBug:
		if parentType != IssueTypeStory && parentType != IssueTypeEpic {
			return errors.New("task/bug parent must be epic or story")
		}
	}
	return nil
}
