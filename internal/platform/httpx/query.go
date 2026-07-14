package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func Pagination(r *http.Request, defaultLimit, maxLimit int) (int, int, error) {
	if r == nil || defaultLimit < 1 || maxLimit < defaultLimit {
		return 0, 0, errors.New("invalid pagination configuration")
	}
	limit, offset := defaultLimit, 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > maxLimit {
			return 0, 0, fmt.Errorf("limit must be between 1 and %d", maxLimit)
		}
		limit = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		offset = value
	}
	return limit, offset, nil
}

func OptionalPositiveInt64(r *http.Request, key string) (int64, bool, error) {
	if r == nil {
		return 0, false, errors.New("request is required")
	}
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, true, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, true, nil
}
