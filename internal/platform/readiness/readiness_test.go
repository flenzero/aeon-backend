package readiness_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/readiness"
)

func TestRequiredFailureRejectsReadinessAndReportsEveryCheck(t *testing.T) {
	probe := readiness.New("economy-api",
		readiness.Required("postgres", func(context.Context) error { return errors.New("connection refused") }),
		readiness.Optional("redis", func(context.Context) error { return errors.New("fallback active") }),
	)
	recorder := httptest.NewRecorder()
	probe.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	var body readiness.Report
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Ready || len(body.Checks) != 2 {
		t.Fatalf("report = %+v", body)
	}
	if body.Checks[0].Status != readiness.StatusFailed || body.Checks[0].Reason != "connection refused" {
		t.Fatalf("required check = %+v", body.Checks[0])
	}
	if body.Checks[1].Status != readiness.StatusWarning || body.Checks[1].Reason != "fallback active" {
		t.Fatalf("optional check = %+v", body.Checks[1])
	}
}

func TestRequiredDatabaseReportsMemoryAdapterAsNotReady(t *testing.T) {
	cfg := config.Config{
		ServiceName:           "account-api",
		Profile:               config.ProfileDevelopment,
		TestScope:             config.TestScopeContract,
		RequiredSchemaVersion: "schema-v1",
	}
	probe := readiness.New(cfg.ServiceName, readiness.PersistenceChecks(cfg, struct{}{})...)
	report := probe.Check(context.Background())

	if report.Ready || len(report.Checks) != 1 || report.Checks[0].Status != readiness.StatusFailed {
		t.Fatalf("report = %+v", report)
	}
	if report.Checks[0].Name != "postgres" || report.Checks[0].Reason == "" {
		t.Fatalf("persistence check = %+v", report.Checks[0])
	}
}
