package issues

import (
"context"

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
issue := NewIssue(orgID, input.ProjectID, actorID, input.Title, input.Description, input.Priority)
created, err := s.repo.Create(ctx, issue)
if err != nil {
return Issue{}, err
}
_ = s.bus.Publish(ctx, events.New(orgID, "issue.created", actorID, map[string]any{"issue": created}))
return created, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Issue, error) { return s.repo.List(ctx, orgID) }

func (s *Service) Get(ctx context.Context, orgID, issueID uuid.UUID) (Issue, error) { return s.repo.GetByID(ctx, orgID, issueID) }

func (s *Service) Update(ctx context.Context, orgID, actorID, issueID uuid.UUID, input UpdateIssueInput) (Issue, error) {
issue, err := s.repo.Update(ctx, orgID, issueID, input)
if err != nil {
return Issue{}, err
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
