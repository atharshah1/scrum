package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/atharshah1/scrum/scrumX/backend/internal/issues"
	"github.com/atharshah1/scrum/scrumX/backend/pkg/cache"
)

type IssueDraft struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Priority    string `json:"priority"`
}

type Suggestion struct {
	Priority string   `json:"priority"`
	Labels   []string `json:"labels"`
	Type     string   `json:"type"`
}

type Service struct {
	llm     LLMClient
	cache   *cache.TTLCache
	timeout time.Duration
}

const defaultAITimeout = 3 * time.Second

func NewService(llm LLMClient, sharedCache *cache.TTLCache, timeout time.Duration) *Service {
	if timeout <= 0 {
		timeout = defaultAITimeout
	}
	return &Service{llm: llm, cache: sharedCache, timeout: timeout}
}

func (s *Service) GenerateIssuesFromText(ctx context.Context, text string) ([]IssueDraft, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	withTimeout, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	prompt := fmt.Sprintf(`Break this into structured issues.
Return JSON array with:
- title
- description
- type (epic/story/task/bug)
- priority (low/medium/high)

Text:
%s`, text)

	drafts, err := s.llm.GenerateIssues(withTimeout, prompt)
	if err != nil || len(drafts) == 0 {
		return normalizeDrafts(heuristicDrafts(text)), nil
	}
	return normalizeDrafts(drafts), nil
}

func (s *Service) SummarizeIssue(ctx context.Context, issue issues.Issue, comments []issues.IssueComment) (string, error) {
	contentHash := hashPayload(struct {
		Title       string                `json:"title"`
		Description string                `json:"description"`
		Comments    []issues.IssueComment `json:"comments"`
	}{
		Title:       issue.Title,
		Description: issue.Description,
		Comments:    comments,
	})
	cacheKey := "ai:summary:" + issue.ID.String() + ":" + contentHash
	if cached, ok := s.readCachedString(cacheKey); ok {
		return cached, nil
	}

	withTimeout, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	prompt := fmt.Sprintf(`Summarize this issue in 2-3 lines.

Title: %s
Description: %s
Comments: %v`, issue.Title, issue.Description, comments)

	summary, err := s.llm.GenerateText(withTimeout, prompt)
	if err != nil || strings.TrimSpace(summary) == "" {
		summary = heuristicSummary(issue, comments)
	}
	summary = strings.TrimSpace(summary)
	if summary == "" {
		summary = "No summary available."
	}
	if s.cache != nil {
		s.cache.Set(cacheKey, summary)
	}
	return summary, nil
}

func (s *Service) SuggestFields(ctx context.Context, title, desc string) (*Suggestion, error) {
	title = strings.TrimSpace(title)
	desc = strings.TrimSpace(desc)
	if title == "" && desc == "" {
		return nil, fmt.Errorf("title or description is required")
	}

	cacheKey := "ai:suggest:" + hashPayload(map[string]string{"title": title, "description": desc})
	if cached, ok := s.readCachedSuggestion(cacheKey); ok {
		return cached, nil
	}

	withTimeout, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	prompt := fmt.Sprintf(`Given this issue, suggest:
- priority (low/medium/high)
- labels (array)
- type (epic/story/task/bug)

Title: %s
Description: %s`, title, desc)

	suggestion, err := s.llm.GenerateSuggestion(withTimeout, prompt)
	if err != nil || suggestion == nil {
		suggestion = heuristicSuggestion(title, desc)
	}
	suggestion = normalizeSuggestion(suggestion)
	if s.cache != nil {
		s.cache.Set(cacheKey, suggestion)
	}
	return suggestion, nil
}

func normalizeDrafts(drafts []IssueDraft) []IssueDraft {
	result := make([]IssueDraft, 0, len(drafts))
	for _, draft := range drafts {
		title := strings.TrimSpace(draft.Title)
		if title == "" {
			continue
		}
		result = append(result, IssueDraft{
			Title:       title,
			Description: strings.TrimSpace(draft.Description),
			Type:        normalizeType(draft.Type),
			Priority:    normalizePriority(draft.Priority),
		})
	}
	if len(result) == 0 {
		return []IssueDraft{{
			Title:    "Review and refine requested work",
			Type:     issues.IssueTypeTask,
			Priority: "medium",
		}}
	}
	return result
}

func normalizeSuggestion(s *Suggestion) *Suggestion {
	if s == nil {
		return &Suggestion{Priority: "medium", Labels: []string{"triage"}, Type: issues.IssueTypeTask}
	}
	labels := make([]string, 0, len(s.Labels))
	seen := map[string]struct{}{}
	for _, label := range s.Labels {
		l := strings.ToLower(strings.TrimSpace(label))
		if l == "" {
			continue
		}
		if _, exists := seen[l]; exists {
			continue
		}
		seen[l] = struct{}{}
		labels = append(labels, l)
	}
	if len(labels) == 0 {
		labels = []string{"triage"}
	}
	return &Suggestion{
		Priority: normalizePriority(s.Priority),
		Labels:   labels,
		Type:     normalizeType(s.Type),
	}
}

func normalizeType(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case issues.IssueTypeEpic:
		return issues.IssueTypeEpic
	case issues.IssueTypeStory:
		return issues.IssueTypeStory
	case issues.IssueTypeBug:
		return issues.IssueTypeBug
	default:
		return issues.IssueTypeTask
	}
}

func normalizePriority(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "low":
		return "low"
	case "high":
		return "high"
	default:
		return "medium"
	}
}

func heuristicDrafts(text string) []IssueDraft {
	splitter := regexp.MustCompile(`(?i)\s*(?:,| and | then |;)\s*`)
	parts := splitter.Split(text, -1)
	drafts := make([]IssueDraft, 0, len(parts))
	for _, part := range parts {
		clean := strings.TrimSpace(strings.Trim(part, "."))
		if clean == "" {
			continue
		}
		drafts = append(drafts, IssueDraft{
			Title:       toSentence(clean),
			Description: clean,
			Type:        inferType(clean),
			Priority:    inferPriority(clean),
		})
	}
	if len(drafts) == 0 {
		return []IssueDraft{{
			Title:       toSentence(text),
			Description: strings.TrimSpace(text),
			Type:        inferType(text),
			Priority:    inferPriority(text),
		}}
	}
	return drafts
}

func heuristicSummary(issue issues.Issue, comments []issues.IssueComment) string {
	base := strings.TrimSpace(issue.Description)
	if base == "" {
		base = "This issue tracks implementation details for the requested work."
	}
	title := strings.TrimSpace(issue.Title)
	if title == "" {
		title = "Issue"
	}
	summary := fmt.Sprintf("%s: %s", title, base)
	if len(comments) > 0 {
		summary += fmt.Sprintf(" There are %d comments with additional context.", len(comments))
	}
	return summary
}

func heuristicSuggestion(title, desc string) *Suggestion {
	text := strings.ToLower(strings.TrimSpace(title + " " + desc))
	labels := []string{"triage"}
	if strings.Contains(text, "payment") {
		labels = append(labels, "payments")
	}
	if strings.Contains(text, "oauth") || strings.Contains(text, "auth") || strings.Contains(text, "login") {
		labels = append(labels, "auth")
	}
	return normalizeSuggestion(&Suggestion{
		Priority: inferPriority(text),
		Labels:   labels,
		Type:     inferType(text),
	})
}

func inferPriority(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "critical"), strings.Contains(t, "urgent"), strings.Contains(t, "failing"), strings.Contains(t, "bug"), strings.Contains(t, "outage"):
		return "high"
	case strings.Contains(t, "cleanup"), strings.Contains(t, "docs"), strings.Contains(t, "minor"):
		return "low"
	default:
		return "medium"
	}
}

func inferType(text string) string {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "bug"), strings.Contains(t, "fix"), strings.Contains(t, "failure"), strings.Contains(t, "error"):
		return issues.IssueTypeBug
	case strings.Contains(t, "epic"), strings.Contains(t, "roadmap"):
		return issues.IssueTypeEpic
	case strings.Contains(t, "story"), strings.Contains(t, "user"):
		return issues.IssueTypeStory
	default:
		return issues.IssueTypeTask
	}
}

func toSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "Untitled issue"
	}
	runes := []rune(text)
	if len(runes) == 0 {
		return "Untitled issue"
	}
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}

func hashPayload(v any) string {
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (s *Service) readCachedString(key string) (string, bool) {
	if s.cache == nil {
		return "", false
	}
	cached, ok := s.cache.Get(key)
	if !ok {
		return "", false
	}
	switch typed := cached.(type) {
	case string:
		return typed, true
	default:
		return "", false
	}
}

func (s *Service) readCachedSuggestion(key string) (*Suggestion, bool) {
	if s.cache == nil {
		return nil, false
	}
	cached, ok := s.cache.Get(key)
	if !ok {
		return nil, false
	}
	switch typed := cached.(type) {
	case *Suggestion:
		return normalizeSuggestion(typed), true
	case map[string]any:
		raw, _ := json.Marshal(typed)
		var parsed Suggestion
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, false
		}
		return normalizeSuggestion(&parsed), true
	default:
		return nil, false
	}
}
