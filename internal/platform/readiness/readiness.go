package readiness

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Status string

const (
	StatusOK      Status = "ok"
	StatusFailed  Status = "failed"
	StatusWarning Status = "warning"
)

type Result struct {
	Name     string `json:"name"`
	Status   Status `json:"status"`
	Required bool   `json:"required"`
	Reason   string `json:"reason,omitempty"`
}

type Report struct {
	Ready   bool     `json:"ready"`
	Service string   `json:"service"`
	Checks  []Result `json:"checks"`
}

type Check struct {
	name     string
	required bool
	run      func(context.Context) error
}

type Probe struct {
	service string
	checks  []Check
	timeout time.Duration
}

func Required(name string, run func(context.Context) error) Check {
	return Check{name: name, required: true, run: run}
}

func Optional(name string, run func(context.Context) error) Check {
	return Check{name: name, run: run}
}

func New(service string, checks ...Check) Probe {
	return Probe{service: service, checks: checks, timeout: 3 * time.Second}
}

func (p Probe) Check(ctx context.Context) Report {
	report := Report{Ready: true, Service: p.service, Checks: make([]Result, 0, len(p.checks))}
	for _, check := range p.checks {
		result := Result{Name: check.name, Status: StatusOK, Required: check.required}
		checkCtx, cancel := context.WithTimeout(ctx, p.timeout)
		err := check.run(checkCtx)
		cancel()
		if err != nil {
			result.Reason = err.Error()
			if check.required {
				result.Status = StatusFailed
				report.Ready = false
			} else {
				result.Status = StatusWarning
			}
		}
		report.Checks = append(report.Checks, result)
	}
	return report
}

func (p Probe) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		report := p.Check(r.Context())
		status := http.StatusOK
		if !report.Ready {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(report)
	})
}
