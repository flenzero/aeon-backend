package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadLocalEnvFile loads KEY=VALUE pairs from local.env at the repo root when
// not running in production. Existing process env vars always win.
func loadLocalEnvFile() {
	profile := strings.TrimSpace(os.Getenv("APP_PROFILE"))
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("AEONBLIGHT_ENV"))
	}
	if strings.EqualFold(profile, "production") || strings.EqualFold(profile, "staging") {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AEONBLIGHT_SKIP_LOCAL_ENV")), "true") {
		return
	}
	path := findLocalEnvPath()
	if path == "" {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func findLocalEnvPath() string {
	candidates := []string{"local.env"}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(dir, "local.env"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for _, path := range candidates {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	return ""
}
