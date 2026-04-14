package issues

import (
	"context"
	"errors"
	"fmt"
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
	if err := validateIssueType(input.IssueType); err != nil {
		return Issue{}, err
	}

	projectOwned, err := s.repo.ProjectBelongsToOrg(ctx, orgID, input.ProjectID)
	if err != nil {
		return Issue{}, err
	}
	if !projectOwned {
		return Issue{}, errors.New("project does not belong to organization")
	}

	if input.AssigneeID != nil {
		assigneeOwned, err := s.repo.UserBelongsToOrg(ctx, orgID, *input.AssigneeID)
		if err != nil {
			return Issue{}, err
		}
		if !assigneeOwned {
			return Issue{}, errors.New("assignee does not belong to organization")
		}
	}

	if input.ParentID != nil {
		parent, err := s.repo.GetByID(ctx, orgID, *input.ParentID)
		if err != nil {
			return Issue{}, err
		}
		if parent.ProjectID != input.ProjectID {
			return Issue{}, errors.New("parent issue must belong to the same project")
		}
		if err := validateHierarchy(strings.ToLower(input.IssueType), strings.ToLower(parent.IssueType)); err != nil {
			return Issue{}, err
		}
	}

	issue := NewIssue(orgID, input.ProjectID, actorID, input)

	if input.SprintID != nil {
		ok, err := s.repo.SprintBelongsToProject(ctx, orgID, *input.SprintID, issue.ProjectID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("sprint does not belong to issue project")
		}
	}

	created, err := s.repo.Create(ctx, issue)
	if err != nil {
		return Issue{}, err
	}
	for _, label := range input.Labels {
		if strings.TrimSpace(label) == "" {
			continue
		}
		if err := s.repo.AddLabel(ctx, orgID, created.ID, label); err != nil {
			return Issue{}, err
		}
	}
	created, err = s.repo.GetByID(ctx, orgID, created.ID)
	if err != nil {
		return Issue{}, err
	}

	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.created", created.ID, map[string]any{"status": created.Status})
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.created", actorID, map[string]any{"issue": created}))
	return created, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, input ListIssuesInput) ([]Issue, error) {
	return s.repo.List(ctx, orgID, input)
}

func (s *Service) Get(ctx context.Context, orgID, issueID uuid.UUID) (Issue, error) {
	return s.repo.GetByID(ctx, orgID, issueID)
}

func (s *Service) Update(ctx context.Context, orgID, actorID, issueID uuid.UUID, input UpdateIssueInput) (Issue, error) {
	current, err := s.repo.GetByID(ctx, orgID, issueID)
	if err != nil {
		return Issue{}, err
	}

	if input.Status != nil {
		if !isAllowedTransition(current.Status, *input.Status) {
			return Issue{}, fmt.Errorf("invalid transition: %s -> %s", current.Status, *input.Status)
		}
	}

	if input.AssigneeID != nil {
		ok, err := s.repo.UserBelongsToOrg(ctx, orgID, *input.AssigneeID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("assignee does not belong to organization")
		}
	}

	if input.SprintID != nil {
		ok, err := s.repo.SprintBelongsToProject(ctx, orgID, *input.SprintID, current.ProjectID)
		if err != nil {
			return Issue{}, err
		}
		if !ok {
			return Issue{}, errors.New("sprint does not belong to issue project")
		}
	}

	issue, err := s.repo.Update(ctx, orgID, issueID, input)
	if err != nil {
		return Issue{}, err
	}

	if input.Status != nil {
		_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.status_changed", issue.ID, map[string]any{"from": current.Status, "to": issue.Status})
	}
	if input.AssigneeID != nil {
		_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.assigned", issue.ID, map[string]any{"assignee_id": input.AssigneeID.String()})
	}
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.updated", actorID, map[string]any{"issue": issue}))
	return issue, nil
}

func (s *Service) Delete(ctx context.Context, orgID, actorID, issueID uuid.UUID) error {
	if err := s.repo.Delete(ctx, orgID, issueID); err != nil {
		return err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.deleted", issueID, nil)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.deleted", actorID, map[string]any{"issue_id": issueID}))
	return nil
}

func (s *Service) Assign(ctx context.Context, orgID, actorID, issueID, assigneeID uuid.UUID) (Issue, error) {
	ok, err := s.repo.UserBelongsToOrg(ctx, orgID, assigneeID)
	if err != nil {
		return Issue{}, err
	}
	if !ok {
		return Issue{}, errors.New("assignee does not belong to organization")
	}
	issue, err := s.repo.Assign(ctx, orgID, issueID, assigneeID)
	if err != nil {
		return Issue{}, err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.assigned", issueID, map[string]any{"assignee_id": assigneeID.String()})
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.assigned", actorID, map[string]any{"issue_id": issueID, "assignee_id": assigneeID}))
	return issue, nil
}

func (s *Service) AddComment(ctx context.Context, orgID, actorID, issueID uuid.UUID, body string) (IssueComment, error) {
	comment, err := s.repo.AddComment(ctx, orgID, issueID, actorID, body)
	if err != nil {
		return IssueComment{}, err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.comment_added", issueID, map[string]any{"comment_id": comment.ID.String()})
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.comment_added", actorID, map[string]any{"issue_id": issueID, "comment": comment}))
	return comment, nil
}

func (s *Service) UpdateComment(ctx context.Context, orgID, actorID, issueID, commentID uuid.UUID, body string) (IssueComment, error) {
	comment, err := s.repo.UpdateComment(ctx, orgID, issueID, commentID, body)
	if err != nil {
		return IssueComment{}, err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.comment_updated", issueID, map[string]any{"comment_id": commentID.String()})
	return comment, nil
}

func (s *Service) DeleteComment(ctx context.Context, orgID, actorID, issueID, commentID uuid.UUID) error {
	if err := s.repo.DeleteComment(ctx, orgID, issueID, commentID); err != nil {
		return err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.comment_deleted", issueID, map[string]any{"comment_id": commentID.String()})
	return nil
}

func (s *Service) AddLabel(ctx context.Context, orgID, actorID, issueID uuid.UUID, label string) error {
	if err := s.repo.AddLabel(ctx, orgID, issueID, label); err != nil {
		return err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.label_added", issueID, map[string]any{"label": label})
	return nil
}

func (s *Service) RemoveLabel(ctx context.Context, orgID, actorID, issueID uuid.UUID, label string) error {
	if err := s.repo.RemoveLabel(ctx, orgID, issueID, label); err != nil {
		return err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.label_removed", issueID, map[string]any{"label": label})
	return nil
}

func (s *Service) LinkIssue(ctx context.Context, orgID, actorID, issueID uuid.UUID, input CreateIssueLinkInput) (IssueLink, error) {
	if issueID == input.RelatedIssueID {
		return IssueLink{}, errors.New("cannot link issue to itself")
	}
	if err := validateRelationType(input.RelationType); err != nil {
		return IssueLink{}, err
	}

	left, err := s.repo.GetByID(ctx, orgID, issueID)
	if err != nil {
		return IssueLink{}, err
	}
	right, err := s.repo.GetByID(ctx, orgID, input.RelatedIssueID)
	if err != nil {
		return IssueLink{}, err
	}
	if left.ProjectID != right.ProjectID {
		return IssueLink{}, errors.New("linked issues must belong to the same project")
	}

	link, err := s.repo.AddLink(ctx, orgID, issueID, input.RelatedIssueID, strings.ToLower(input.RelationType))
	if err != nil {
		return IssueLink{}, err
	}
	_ = s.repo.LogActivity(ctx, orgID, actorID, "issue.link_added", issueID, map[string]any{"link_id": link.ID.String()})
	return link, nil
}

func (s *Service) ListDependencies(ctx context.Context, orgID, issueID uuid.UUID) ([]IssueLink, error) {
	return s.repo.ListLinks(ctx, orgID, issueID)
}

func validateRelationType(relation string) error {
	switch strings.ToLower(strings.TrimSpace(relation)) {
	case "blocks", "is_blocked_by", "relates_to", "duplicates":
		return nil
	default:
		return errors.New("invalid relation_type")
	}
}

func validateIssueType(issueType string) error {
	switch strings.ToLower(strings.TrimSpace(issueType)) {
	case "", "epic", "story", "task", "bug":
		return nil
	default:
		return errors.New("invalid issue_type")
	}
}

func validateHierarchy(childType, parentType string) error {
	switch childType {
	case "story":
		if parentType != "epic" {
			return errors.New("story issues must have an epic parent")
		}
	case "task", "bug":
		if parentType != "story" {
			return errors.New("task and bug issues must have a story parent")
		}
	case "epic":
		return errors.New("epic issues cannot have a parent")
	}
	return nil
}

func isAllowedTransition(current, next string) bool {
	current = strings.ToLower(strings.TrimSpace(current))
	next = strings.ToLower(strings.TrimSpace(next))
	if current == next {
		return true
	}
	allowed := map[string]map[string]bool{
		"todo":        {"in_progress": true},
		"in_progress": {"in_review": true, "done": true, "todo": true},
		"in_review":   {"done": true, "in_progress": true},
		"done":        {"in_progress": true},
	}
	return allowed[current][next]
}
