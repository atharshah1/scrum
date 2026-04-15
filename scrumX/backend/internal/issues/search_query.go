package issues

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type IssueSearchCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

type IssueSearchAST struct {
	Type       string                 `json:"type"`
	Conditions []IssueSearchCondition `json:"conditions,omitempty"`
	Clauses    []IssueSearchAST       `json:"clauses,omitempty"`
}

type issueSearchExpr struct {
	kind      string
	condition IssueSearchCondition
	left      *issueSearchExpr
	right     *issueSearchExpr
}

type issueSearchParser struct {
	tokens []string
	pos    int
}

func parseIssueSearchQuery(raw string) (*issueSearchExpr, IssueSearchAST, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	if len(tokens) == 0 {
		return nil, IssueSearchAST{}, fmt.Errorf("query is required")
	}
	parser := issueSearchParser{tokens: tokens}
	expr, err := parser.parseExpression()
	if err != nil {
		return nil, IssueSearchAST{}, err
	}
	if parser.pos != len(tokens) {
		return nil, IssueSearchAST{}, fmt.Errorf("unexpected token: %s", tokens[parser.pos])
	}
	return expr, expr.toAST(), nil
}

func (p *issueSearchParser) parseExpression() (*issueSearchExpr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.hasToken() && strings.EqualFold(p.peek(), "OR") {
		p.pos++
		right, parseErr := p.parseAnd()
		if parseErr != nil {
			return nil, parseErr
		}
		left = &issueSearchExpr{kind: "OR", left: left, right: right}
	}
	return left, nil
}

func (p *issueSearchParser) parseAnd() (*issueSearchExpr, error) {
	left, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	for p.hasToken() && strings.EqualFold(p.peek(), "AND") {
		p.pos++
		right, parseErr := p.parseCondition()
		if parseErr != nil {
			return nil, parseErr
		}
		left = &issueSearchExpr{kind: "AND", left: left, right: right}
	}
	return left, nil
}

func (p *issueSearchParser) parseCondition() (*issueSearchExpr, error) {
	if !p.hasToken() {
		return nil, fmt.Errorf("expected condition")
	}
	token := p.peek()
	if strings.EqualFold(token, "AND") || strings.EqualFold(token, "OR") {
		return nil, fmt.Errorf("expected condition before %s", token)
	}
	p.pos++
	parts := strings.SplitN(token, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid condition: %s (expected field=value)", token)
	}
	field := strings.ToLower(strings.TrimSpace(parts[0]))
	value := strings.TrimSpace(parts[1])
	if field == "" || value == "" {
		return nil, fmt.Errorf("invalid condition: %s (expected non-empty field and value)", token)
	}
	return &issueSearchExpr{
		kind: "COND",
		condition: IssueSearchCondition{
			Field: field,
			Op:    "=",
			Value: value,
		},
	}, nil
}

func (p *issueSearchParser) hasToken() bool {
	return p.pos < len(p.tokens)
}

func (p *issueSearchParser) peek() string {
	return p.tokens[p.pos]
}

func (e *issueSearchExpr) toAST() IssueSearchAST {
	if e == nil {
		return IssueSearchAST{}
	}
	switch e.kind {
	case "COND":
		return IssueSearchAST{
			Type:       "CONDITION",
			Conditions: []IssueSearchCondition{e.condition},
		}
	case "AND", "OR":
		conditions, ok := collectFlatConditions(e, e.kind)
		if ok {
			return IssueSearchAST{
				Type:       e.kind,
				Conditions: conditions,
			}
		}
		return IssueSearchAST{
			Type:    e.kind,
			Clauses: []IssueSearchAST{e.left.toAST(), e.right.toAST()},
		}
	default:
		return IssueSearchAST{}
	}
}

func collectFlatConditions(expr *issueSearchExpr, kind string) ([]IssueSearchCondition, bool) {
	if expr == nil {
		return nil, false
	}
	if expr.kind == "COND" {
		return []IssueSearchCondition{expr.condition}, true
	}
	if expr.kind != kind {
		return nil, false
	}
	left, ok := collectFlatConditions(expr.left, kind)
	if !ok {
		return nil, false
	}
	right, ok := collectFlatConditions(expr.right, kind)
	if !ok {
		return nil, false
	}
	return append(left, right...), true
}

func buildIssueSearchSQL(expr *issueSearchExpr, actorID uuid.UUID, args *[]any, argN *int) (string, error) {
	if expr == nil {
		return "", fmt.Errorf("empty search expression")
	}
	switch expr.kind {
	case "AND", "OR":
		left, err := buildIssueSearchSQL(expr.left, actorID, args, argN)
		if err != nil {
			return "", err
		}
		right, err := buildIssueSearchSQL(expr.right, actorID, args, argN)
		if err != nil {
			return "", err
		}
		return "(" + left + " " + expr.kind + " " + right + ")", nil
	case "COND":
		return buildIssueConditionSQL(expr.condition, actorID, args, argN)
	default:
		return "", fmt.Errorf("invalid expression node")
	}
}

func buildIssueConditionSQL(condition IssueSearchCondition, actorID uuid.UUID, args *[]any, argN *int) (string, error) {
	field := strings.ToLower(strings.TrimSpace(condition.Field))
	value := strings.TrimSpace(condition.Value)
	if condition.Op != "=" {
		return "", fmt.Errorf("unsupported operator for %s: %s", field, condition.Op)
	}

	placeholder := func(v any) string {
		*args = append(*args, v)
		token := "$" + itoa(*argN)
		*argN = *argN + 1
		return token
	}

	switch field {
	case "status":
		return "i.status = " + placeholder(strings.ToLower(value)), nil
	case "priority":
		return "i.priority = " + placeholder(strings.ToLower(value)), nil
	case "type":
		return "i.issue_type = " + placeholder(strings.ToLower(value)), nil
	case "labels", "label":
		return "EXISTS (SELECT 1 FROM issue_labels l WHERE l.org_id=i.org_id AND l.issue_id=i.id AND l.label=" + placeholder(strings.ToLower(value)) + ")", nil
	case "sprint":
		sprintID, err := uuid.Parse(value)
		if err != nil {
			return "", fmt.Errorf("invalid sprint value: %s", value)
		}
		return "i.sprint_id = " + placeholder(sprintID), nil
	case "assignee":
		resolved := strings.ToLower(value)
		if resolved == "me" || resolved == "mine" {
			if actorID == uuid.Nil {
				return "", fmt.Errorf("assignee aliases me/mine require authenticated user")
			}
			return "i.assignee_id = " + placeholder(actorID), nil
		}
		assigneeID, err := uuid.Parse(value)
		if err != nil {
			return "", fmt.Errorf("invalid assignee value: %s", value)
		}
		return "i.assignee_id = " + placeholder(assigneeID), nil
	default:
		return "", fmt.Errorf("unsupported search field: %s", condition.Field)
	}
}
