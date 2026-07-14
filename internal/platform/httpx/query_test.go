package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestPaginationRejectsExtremeAndMalformedValues(t *testing.T) {
	tests := []struct {
		target             string
		wantLimit, wantOff int
		wantErr            bool
	}{
		{target: "/", wantLimit: 20, wantOff: 0},
		{target: "/?limit=100&offset=2147483647", wantLimit: 100, wantOff: 2147483647},
		{target: "/?limit=0", wantErr: true},
		{target: "/?limit=101", wantErr: true},
		{target: "/?offset=-1", wantErr: true},
		{target: "/?limit=999999999999999999999999", wantErr: true},
		{target: "/?offset=abc", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.target, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.target, nil)
			limit, offset, err := Pagination(req, 20, 100)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Pagination() error=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && (limit != tc.wantLimit || offset != tc.wantOff) {
				t.Fatalf("Pagination()=(%d,%d), want=(%d,%d)", limit, offset, tc.wantLimit, tc.wantOff)
			}
		})
	}
}

func TestOptionalPositiveInt64RejectsMalformedAndNonPositiveValues(t *testing.T) {
	for _, target := range []string{"/?accountId=abc", "/?accountId=-1", "/?accountId=0", "/?accountId=999999999999999999999999"} {
		req := httptest.NewRequest("GET", target, nil)
		if _, _, err := OptionalPositiveInt64(req, "accountId"); err == nil {
			t.Fatalf("OptionalPositiveInt64(%q) accepted invalid value", target)
		}
	}
	req := httptest.NewRequest("GET", "/", nil)
	if value, present, err := OptionalPositiveInt64(req, "accountId"); err != nil || present || value != 0 {
		t.Fatalf("missing value = (%d,%v,%v)", value, present, err)
	}
}
