package issues

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/atharshah1/scrum/scrumX/backend/internal/authz"
	"github.com/atharshah1/scrum/scrumX/backend/internal/events"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
	"github.com/google/uuid"
)

type Service struct {
	repo  *Repository
	bus   events.Publisher
	authz *authz.Service
	cache *cache.TTLCache
}

func NewService(repo *Repository, bus events.Publisher, authzService *authz.Service, sharedCache *cache.TTLCache) *Service {
	return &Service{repo: repo, bus: bus, authz: authzService, cache: sharedCache}
}

func (s *Service) Create(ctx context.Context, orgID, actorID uuid.UUID, input CreateIssueInput) (Issue, error) {
	if err := s.requireWriteProject(ctx, orgID, actorID, input.ProjectID); err != nil {
		return Issue{}, err
	}
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
	s.invalidateProjectCaches(orgID, input.ProjectID)
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
	if err := s.requireWriteProject(ctx, orgID, actorID, current.ProjectID); err != nil {
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
		if *input.Status == current.Status {
			input.Status = nil
		}
	}
	return s.applyIssueUpdate(ctx, orgID, actorID, current, input, "updated", "issue.updated")
}

func (s *Service) Delete(ctx context.Context, orgID, actorID, issueID uuid.UUID) error {
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, orgID, issueID); err != nil {
		return err
	}
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.deleted", actorID, map[string]any{"issue_id": issueID}))
	s.invalidateProjectCaches(orgID, projectID)
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
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return err
	}
	circular, err := s.repo.HasCircularRelation(ctx, orgID, issueID, relatedIssueID, relationType)
	if err != nil {
		return err
	}
	if circular {
		return errors.New("relation would create a circular dependency")
	}
	if err := s.repo.AddRelation(ctx, orgID, issueID, relatedIssueID, relationType); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "relation_added", "relation_type", "", relationType)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.relation_added", actorID, map[string]any{"issue_id": issueID, "related_issue_id": relatedIssueID, "relation_type": relationType}))
	s.invalidateProjectCaches(orgID, projectID)
	return nil
}

func (s *Service) ListRelations(ctx context.Context, orgID, issueID uuid.UUID) ([]map[string]any, error) {
	return s.repo.ListRelations(ctx, orgID, issueID)
}

func (s *Service) AddWatcher(ctx context.Context, orgID, actorID, issueID, userID uuid.UUID) error {
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return err
	}
	if err := s.repo.AddWatcher(ctx, orgID, issueID, userID); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "watcher_added", "user_id", "", userID.String())
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.watcher_added", actorID, map[string]any{"issue_id": issueID, "user_id": userID}))
	s.invalidateProjectCaches(orgID, projectID)
	return nil
}

func (s *Service) RemoveWatcher(ctx context.Context, orgID, actorID, issueID, userID uuid.UUID) error {
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return err
	}
	if err := s.repo.RemoveWatcher(ctx, orgID, issueID, userID); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "watcher_removed", "user_id", userID.String(), "")
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.watcher_removed", actorID, map[string]any{"issue_id": issueID, "user_id": userID}))
	s.invalidateProjectCaches(orgID, projectID)
	return nil
}

func (s *Service) ListWatchers(ctx context.Context, orgID, issueID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.ListWatchers(ctx, orgID, issueID)
}

func (s *Service) AddLabel(ctx context.Context, orgID, actorID, issueID uuid.UUID, label string) error {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return errors.New("label is required")
	}
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return err
	}
	if err := s.repo.AddLabel(ctx, orgID, issueID, label); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "label_added", "label", "", label)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.label_added", actorID, map[string]any{"issue_id": issueID, "label": label}))
	s.invalidateProjectCaches(orgID, projectID)
	return nil
}

func (s *Service) RemoveLabel(ctx context.Context, orgID, actorID, issueID uuid.UUID, label string) error {
	label = strings.ToLower(strings.TrimSpace(label))
	if label == "" {
		return errors.New("label is required")
	}
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return err
	}
	if err := s.repo.RemoveLabel(ctx, orgID, issueID, label); err != nil {
		return err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "label_removed", "label", label, "")
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.label_removed", actorID, map[string]any{"issue_id": issueID, "label": label}))
	s.invalidateProjectCaches(orgID, projectID)
	return nil
}

func (s *Service) ListLabels(ctx context.Context, orgID, issueID uuid.UUID) ([]string, error) {
	return s.repo.ListLabels(ctx, orgID, issueID)
}

func (s *Service) CreateComment(ctx context.Context, orgID, actorID, issueID uuid.UUID, body string) (IssueComment, error) {
	if strings.TrimSpace(body) == "" {
		return IssueComment{}, errors.New("body is required")
	}
	projectID, err := s.repo.GetIssueProjectID(ctx, orgID, issueID)
	if err != nil {
		return IssueComment{}, err
	}
	if err := s.requireWriteProject(ctx, orgID, actorID, projectID); err != nil {
		return IssueComment{}, err
	}
	comment, err := s.repo.CreateComment(ctx, orgID, issueID, actorID, body)
	if err != nil {
		return IssueComment{}, err
	}
	_ = s.repo.AddActivity(ctx, orgID, issueID, actorID, "comment_created", "comment", "", body)
	_ = s.bus.Publish(ctx, events.New(orgID, "issue.comment_created", actorID, map[string]any{"issue_id": issueID, "comment": comment}))
	s.invalidateProjectCaches(orgID, projectID)
	return comment, nil
}

func (s *Service) ListComments(ctx context.Context, orgID, issueID uuid.UUID, page, limit int) ([]IssueComment, int, error) {
	return s.repo.ListComments(ctx, orgID, issueID, page, limit)
}

func (s *Service) ListActivities(ctx context.Context, orgID, issueID uuid.UUID, page, limit int) ([]IssueActivity, int, error) {
	return s.repo.ListActivities(ctx, orgID, issueID, page, limit)
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

func (s *Service) requireWriteProject(ctx context.Context, orgID, actorID, projectID uuid.UUID) error {
	if actorID == uuid.Nil {
		return errors.New("missing actor")
	}
	role, err := s.authz.ResolveProjectRole(ctx, orgID, actorID, projectID)
	if err != nil {
		return err
	}
	if !authz.CanWrite(role) {
		return errors.New("forbidden")
	}
	return nil
}

func applyTransitionRule(input *UpdateIssueInput, current Issue, actorID uuid.UUID, rule WorkflowTransitionRule) error {
	if raw, exists := rule.Conditions["assignee_only"]; exists {
		assigneeOnly, ok := raw.(bool)
		if !ok {
			return errors.New("invalid transition condition: assignee_only must be boolean")
		}
		if assigneeOnly {
			if current.AssigneeID == nil || *current.AssigneeID != actorID {
				return errors.New("only assignee can perform this transition")
			}
		}
	}
	if reqRaw, ok := rule.Validators["required_fields"]; ok {
		reqFields, ok := reqRaw.([]any)
		if ok {
			for _, rf := range reqFields {
				field, _ := rf.(string)
				switch field {
				case "assignee_id":
					if current.AssigneeID == nil && (input.AssigneeID == nil || *input.AssigneeID == uuid.Nil) {
						return fmt.Errorf("validator failed: %s is required", field)
					}
				default:
					return fmt.Errorf("unsupported validator field: %s", field)
				}
			}
		}
	}
	if assignToActor, _ := rule.PostFunctions["assign_to_actor"].(bool); assignToActor && actorID != uuid.Nil {
		input.AssigneeID = &actorID
	}
	return nil
}

func (s *Service) ApplyAutomationUpdate(ctx context.Context, orgID, issueID uuid.UUID, status string, assigneeID *uuid.UUID) error {
	current, err := s.repo.GetByID(ctx, orgID, issueID)
	if err != nil {
		return err
	}
	input := UpdateIssueInput{}
	if strings.TrimSpace(status) != "" {
		normalizedStatus := strings.ToLower(strings.TrimSpace(status))
		input.Status = &normalizedStatus
	}
	if assigneeID != nil {
		input.AssigneeID = assigneeID
	}
	_, err = s.applyIssueUpdate(ctx, orgID, uuid.Nil, current, input, "automation_updated", "issue.automated")
	return err
}

func (s *Service) invalidateProjectCaches(orgID, projectID uuid.UUID) {
	if s.cache == nil {
		return
	}
	// Project-scoped board and issue caches. Board keys embed the projectID so we
	// only evict entries for the affected project rather than the whole org.
	s.cache.DeletePrefix("board:" + orgID.String() + ":" + projectID.String() + ":")
	// Issue list keys encode the projectID filter as "p={id}". We invalidate that
	// exact project's cached pages plus unfiltered (projectID==Nil) org-wide pages.
	s.cache.DeletePrefix("issues:" + orgID.String() + ":p=" + projectID.String() + ":")
	s.cache.DeletePrefix("issues:" + orgID.String() + ":p=" + uuid.Nil.String() + ":")
}

func (s *Service) applyIssueUpdate(ctx context.Context, orgID, actorID uuid.UUID, current Issue, input UpdateIssueInput, activityAction, eventType string) (Issue, error) {
	if input.Status != nil {
		rule, err := s.repo.GetTransitionRule(ctx, orgID, current.ProjectID, current.Status, *input.Status)
		if err != nil {
			return Issue{}, err
		}
		if !rule.Allowed {
			return Issue{}, errors.New("invalid status transition")
		}
		if err := applyTransitionRule(&input, current, actorID, rule); err != nil {
			return Issue{}, err
		}
	}
	issue, err := s.repo.Update(ctx, orgID, current.ID, input)
	if err != nil {
		return Issue{}, err
	}
	_ = s.repo.AddActivity(ctx, orgID, issue.ID, actorID, activityAction, "", "", "")
	if input.Status != nil && *input.Status != current.Status {
		_ = s.repo.AddActivity(ctx, orgID, issue.ID, actorID, "status_changed", "status", current.Status, *input.Status)
	}
	_ = s.bus.Publish(ctx, events.New(orgID, eventType, actorID, map[string]any{"issue": issue}))
	s.invalidateProjectCaches(orgID, current.ProjectID)
	return issue, nil
}
