package validation

import (
	"fmt"
	"strings"
)

func NormalizePagination(page, limit, defaultLimit, maxLimit int) (int, int, error) {
	if page < 1 {
		return 0, 0, fmt.Errorf("page must be >= 1")
	}
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit {
		return 0, 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
	}
	return page, limit, nil
}

func NormalizeRequiredString(field, value string, maxLen int) (string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if maxLen > 0 && len(v) > maxLen {
		return "", fmt.Errorf("%s exceeds max length %d", field, maxLen)
	}
	return v, nil
}

func NormalizeOptionalString(field, value string, maxLen int) (string, error) {
	v := strings.TrimSpace(value)
	if maxLen > 0 && len(v) > maxLen {
		return "", fmt.Errorf("%s exceeds max length %d", field, maxLen)
	}
	return v, nil
}

func NormalizeOptionalEnum(field, value string, allowed map[string]struct{}) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return "", nil
	}
	if _, ok := allowed[v]; !ok {
		return "", fmt.Errorf("invalid %s", field)
	}
	return v, nil
}
