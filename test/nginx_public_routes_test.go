package test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestNginxTemplatesRoutePublicHomepageEndpoints(t *testing.T) {
	templates := []string{
		filepath.Join("..", "deploy", "linux", "nginx", "aeonblight.http.conf.template"),
		filepath.Join("..", "deploy", "linux", "nginx", "aeonblight.ssl.conf.template"),
	}
	expected := []struct {
		name     string
		location string
		upstream string
	}{
		{name: "account public homepage", location: `/api/public/`, upstream: `aeon_account_api`},
		{name: "economy public announcements", location: `/api/announcements/`, upstream: `aeon_economy_api`},
	}

	for _, template := range templates {
		raw, err := os.ReadFile(template)
		if err != nil {
			t.Fatalf("read %s: %v", template, err)
		}
		text := string(raw)
		for _, want := range expected {
			pattern := regexp.MustCompile(`(?s)location\s+\^~\s+` + regexp.QuoteMeta(want.location) + `\s*\{[^}]*proxy_pass\s+http://` + regexp.QuoteMeta(want.upstream) + `\s*;`)
			if !pattern.MatchString(text) {
				t.Fatalf("%s does not route %s to %s", template, want.location, want.upstream)
			}
		}
	}
}
