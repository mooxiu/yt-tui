package main

import (
	"context"
	"testing"

	"github.com/EugeneShtoka/yt-tui/internal/api/apitest"
	"github.com/EugeneShtoka/yt-tui/internal/config"
	"github.com/EugeneShtoka/yt-tui/internal/tui/app"
)

type diagnosticBackend struct {
	apitest.NopBackend
	called bool
}

func (b *diagnosticBackend) CheckAvailability(ctx context.Context) ([]config.ConfigIssue, error) {
	b.called = true
	return []config.ConfigIssue{{Severity: config.SeverityWarning, Message: "test issue"}}, ctx.Err()
}

func TestStartupDiagnosticsAreDeferred(t *testing.T) {
	// No local yt-dlp is needed for this test.
	t.Setenv("PATH", t.TempDir())
	backend := &diagnosticBackend{}
	cmd := startupDiagnosticsCmd(t.Context(), backend)
	if backend.called {
		t.Fatal("creating startup diagnostics must not run checks")
	}
	msg, ok := cmd().(app.StartupDiagnosticsMsg)
	if !ok || !backend.called || len(msg.Issues) != 1 {
		t.Fatalf("command did not deliver background checks: %#v", msg)
	}
}

// TestRunPositionTrackerArgValidation covers the tracker subcommand's argument
// handling — the paths that reject bad input before touching the config or DB.
func TestRunPositionTrackerArgValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no args", nil},
		{"missing pid", []string{"vid1"}},
		{"too many args", []string{"vid1", "123", "extra"}},
		{"non-numeric pid", []string{"vid1", "notapid"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := runPositionTracker(tt.args); err == nil {
				t.Errorf("runPositionTracker(%q): got nil error, want a validation error", tt.args)
			}
		})
	}
}
