package issues

import (
	"fmt"
	"strings"
	"unicode"

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
	depth  int
}

const (
	maxIssueSearchLength  = 500
	maxIssueSearchClauses = 25
	maxIssueSearchDepth   = 5
	maxFuzzyPatternRunes  = 64
)

var supportedIssueSearchFields = map[string]struct{}{
	"status":   {},
	"priority": {},
	"type":     {},
	"labels":   {},
	"label":    {},
	"sprint":   {},
	"assignee": {},
	"title":    {},
	"project":  {},
}

type IssueSearchValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Token   string `json:"token,omitempty"`
}

func (e *IssueSearchValidationError) Error() string { return e.Message }

func parseIssueSearchQuery(raw string) (*issueSearchExpr, IssueSearchAST, error) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return nil, IssueSearchAST{}, &IssueSearchValidationError{
			Code:    "query_required",
			Message: "query is required",
		}
	}
	if len(query) > maxIssueSearchLength {
		return nil, IssueSearchAST{}, &IssueSearchValidationError{
			Code:    "query_too_long",
			Message: fmt.Sprintf("query exceeds maximum length of %d characters", maxIssueSearchLength),
		}
	}
	tokens, err := tokenizeIssueSearchQuery(query)
	if err != nil {
		return nil, IssueSearchAST{}, err
	}
	if len(tokens) == 0 {
		return nil, IssueSearchAST{}, &IssueSearchValidationError{
			Code:    "query_required",
			Message: "query is required",
		}
	}
	parser := issueSearchParser{tokens: tokens}
	expr, err := parser.parseExpression(1)
	if err != nil {
		return nil, IssueSearchAST{}, err
	}
	if parser.pos != len(tokens) {
		return nil, IssueSearchAST{}, &IssueSearchValidationError{
			Code:    "unexpected_token",
			Message: fmt.Sprintf("unexpected token: %s", tokens[parser.pos]),
			Token:   tokens[parser.pos],
		}
	}
	if countConditions(expr) > maxIssueSearchClauses {
		return nil, IssueSearchAST{}, &IssueSearchValidationError{
			Code:    "too_many_clauses",
			Message: fmt.Sprintf("query exceeds maximum of %d clauses", maxIssueSearchClauses),
		}
	}
	return expr, expr.toAST(), nil
}

func (p *issueSearchParser) parseExpression(depth int) (*issueSearchExpr, error) {
	if depth > maxIssueSearchDepth {
		return nil, &IssueSearchValidationError{
			Code:    "max_depth_exceeded",
			Message: fmt.Sprintf("query nesting exceeds maximum depth of %d", maxIssueSearchDepth),
		}
	}
	left, err := p.parseAnd(depth)
	if err != nil {
		return nil, err
	}
	for p.hasToken() && strings.EqualFold(p.peek(), "OR") {
		p.pos++
		right, parseErr := p.parseAnd(depth)
		if parseErr != nil {
			return nil, parseErr
		}
		left = &issueSearchExpr{kind: "OR", left: left, right: right}
	}
	return left, nil
}

func (p *issueSearchParser) parseAnd(depth int) (*issueSearchExpr, error) {
	left, err := p.parsePrimary(depth)
	if err != nil {
		return nil, err
	}
	for p.hasToken() && strings.EqualFold(p.peek(), "AND") {
		p.pos++
		right, parseErr := p.parsePrimary(depth)
		if parseErr != nil {
			return nil, parseErr
		}
		left = &issueSearchExpr{kind: "AND", left: left, right: right}
	}
	return left, nil
}

func (p *issueSearchParser) parsePrimary(depth int) (*issueSearchExpr, error) {
	if p.hasToken() && p.peek() == "(" {
		p.pos++
		expr, err := p.parseExpression(depth + 1)
		if err != nil {
			return nil, err
		}
		if !p.hasToken() || p.peek() != ")" {
			return nil, &IssueSearchValidationError{
				Code:    "missing_closing_paren",
				Message: "missing closing parenthesis",
			}
		}
		p.pos++
		return expr, nil
	}
	return p.parseCondition()
}

func (p *issueSearchParser) parseCondition() (*issueSearchExpr, error) {
	if !p.hasToken() {
		return nil, &IssueSearchValidationError{
			Code:    "expected_condition",
			Message: "expected condition",
		}
	}
	token := p.peek()
	if token == ")" {
		return nil, &IssueSearchValidationError{
			Code:    "unexpected_closing_paren",
			Message: "unexpected closing parenthesis",
			Token:   token,
		}
	}
	if strings.EqualFold(token, "AND") || strings.EqualFold(token, "OR") || token == "(" {
		return nil, &IssueSearchValidationError{
			Code:    "expected_condition",
			Message: fmt.Sprintf("expected condition before %s", token),
			Token:   token,
		}
	}
	p.pos++
	field, op, value, err := parseIssueSearchConditionToken(token)
	if err != nil {
		return nil, err
	}
	if _, ok := supportedIssueSearchFields[field]; !ok {
		return nil, &IssueSearchValidationError{
			Code:    "unsupported_field",
			Message: fmt.Sprintf("unsupported search field: %s", field),
			Field:   field,
			Token:   token,
		}
	}
	if op != "=" && op != "~" {
		return nil, &IssueSearchValidationError{
			Code:    "unsupported_operator",
			Message: fmt.Sprintf("unsupported operator for %s: %s", field, op),
			Field:   field,
			Token:   token,
		}
	}
	return &issueSearchExpr{
		kind: "COND",
		condition: IssueSearchCondition{
			Field: field,
			Op:    op,
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
	op := strings.TrimSpace(condition.Op)
	if op == "" {
		op = "="
	}
	if op != "=" && op != "~" {
		return "", &IssueSearchValidationError{
			Code:    "unsupported_operator",
			Message: fmt.Sprintf("unsupported operator for %s: %s", field, op),
			Field:   field,
		}
	}

	placeholder := func(v any) string {
		*args = append(*args, v)
		token := "$" + itoa(*argN)
		*argN = *argN + 1
		return token
	}

	switch field {
	case "status":
		if op == "~" {
			return "i.status ILIKE " + placeholder(buildFuzzyLikePattern(value)) + " ESCAPE '\\'", nil
		}
		return "i.status = " + placeholder(strings.ToLower(value)), nil
	case "priority":
		if op == "~" {
			return "i.priority ILIKE " + placeholder(buildFuzzyLikePattern(value)) + " ESCAPE '\\'", nil
		}
		return "i.priority = " + placeholder(strings.ToLower(value)), nil
	case "type":
		if op == "~" {
			return "i.issue_type ILIKE " + placeholder(buildFuzzyLikePattern(value)) + " ESCAPE '\\'", nil
		}
		return "i.issue_type = " + placeholder(strings.ToLower(value)), nil
	case "labels", "label":
		if op == "~" {
			return "EXISTS (SELECT 1 FROM issue_labels l WHERE l.org_id=i.org_id AND l.issue_id=i.id AND l.label ILIKE " + placeholder(buildFuzzyLikePattern(strings.ToLower(value))) + " ESCAPE '\\')", nil
		}
		return "EXISTS (SELECT 1 FROM issue_labels l WHERE l.org_id=i.org_id AND l.issue_id=i.id AND l.label=" + placeholder(strings.ToLower(value)) + ")", nil
	case "sprint":
		if op == "~" {
			return "", &IssueSearchValidationError{
				Code:    "unsupported_operator",
				Message: "operator ~ is not supported for sprint",
				Field:   field,
			}
		}
		sprintID, err := uuid.Parse(value)
		if err != nil {
			return "", &IssueSearchValidationError{
				Code:    "invalid_value",
				Message: fmt.Sprintf("invalid sprint value: %s", value),
				Field:   field,
			}
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
		if op == "~" {
			return "", &IssueSearchValidationError{
				Code:    "unsupported_operator",
				Message: "operator ~ is not supported for assignee",
				Field:   field,
			}
		}
		assigneeID, err := uuid.Parse(value)
		if err != nil {
			return "", &IssueSearchValidationError{
				Code:    "invalid_value",
				Message: fmt.Sprintf("invalid assignee value: %s", value),
				Field:   field,
			}
		}
		return "i.assignee_id = " + placeholder(assigneeID), nil
	case "title":
		if op == "~" {
			return "i.title ILIKE " + placeholder(buildFuzzyLikePattern(value)) + " ESCAPE '\\'", nil
		}
		return "i.title = " + placeholder(value), nil
	case "project":
		if op == "~" {
			return "", &IssueSearchValidationError{
				Code:    "unsupported_operator",
				Message: "operator ~ is not supported for project",
				Field:   field,
			}
		}
		projectID, err := uuid.Parse(value)
		if err != nil {
			return "", &IssueSearchValidationError{
				Code:    "invalid_value",
				Message: fmt.Sprintf("invalid project value: %s", value),
				Field:   field,
			}
		}
		return "i.project_id = " + placeholder(projectID), nil
	default:
		return "", &IssueSearchValidationError{
			Code:    "unsupported_field",
			Message: fmt.Sprintf("unsupported search field: %s", condition.Field),
			Field:   condition.Field,
		}
	}
}

func buildFuzzyLikePattern(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "%"
	}
	var b strings.Builder
	b.WriteByte('%')
	lastWildcard := true
	processed := 0
	for _, r := range normalized {
		if processed >= maxFuzzyPatternRunes {
			break
		}
		if unicode.IsSpace(r) {
			if !lastWildcard {
				b.WriteByte('%')
				lastWildcard = true
			}
			continue
		}
		escaped := escapeLikePattern(string(r))
		b.WriteString(escaped)
		b.WriteByte('%')
		lastWildcard = true
		processed++
	}
	return b.String()
}

func tokenizeIssueSearchQuery(raw string) ([]string, error) {
	tokens := make([]string, 0, 16)
	var b strings.Builder
	inQuotes := false
	escaped := false
	for _, r := range raw {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '"':
			inQuotes = !inQuotes
			b.WriteRune(r)
		case !inQuotes && (unicode.IsSpace(r) || r == '(' || r == ')'):
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
			if r == '(' || r == ')' {
				tokens = append(tokens, string(r))
			}
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		return nil, &IssueSearchValidationError{
			Code:    "invalid_escape",
			Message: "query ends with incomplete escape sequence",
		}
	}
	if inQuotes {
		return nil, &IssueSearchValidationError{
			Code:    "unterminated_quote",
			Message: "unterminated quoted string",
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens, nil
}

func parseIssueSearchConditionToken(token string) (field, op, value string, err error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", "", "", &IssueSearchValidationError{
			Code:    "invalid_condition",
			Message: "empty condition token",
		}
	}
	idx := strings.IndexAny(token, "=~")
	if idx <= 0 || idx >= len(token)-1 {
		return "", "", "", &IssueSearchValidationError{
			Code:    "invalid_condition",
			Message: fmt.Sprintf("invalid condition: %s (expected field=value or field~value)", token),
			Token:   token,
		}
	}
	field = strings.ToLower(strings.TrimSpace(token[:idx]))
	op = strings.TrimSpace(token[idx : idx+1])
	value = strings.TrimSpace(token[idx+1:])
	value = unquoteIssueSearchValue(value)
	if field == "" || value == "" {
		return "", "", "", &IssueSearchValidationError{
			Code:    "invalid_condition",
			Message: fmt.Sprintf("invalid condition: %s (expected non-empty field and value)", token),
			Token:   token,
		}
	}
	return field, op, value, nil
}

func unquoteIssueSearchValue(v string) string {
	if len(v) >= 2 && strings.HasPrefix(v, "\"") && strings.HasSuffix(v, "\"") {
		v = strings.TrimPrefix(v, "\"")
		v = strings.TrimSuffix(v, "\"")
		v = strings.ReplaceAll(v, `\"`, `"`)
		v = strings.ReplaceAll(v, `\\`, `\`)
	}
	return strings.TrimSpace(v)
}

func countConditions(expr *issueSearchExpr) int {
	if expr == nil {
		return 0
	}
	if expr.kind == "COND" {
		return 1
	}
	return countConditions(expr.left) + countConditions(expr.right)
}
