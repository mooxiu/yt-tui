package app

import (
	"context"
	"strings"
	"testing"

	"github.com/EugeneShtoka/yt-tui/internal/api/apitest"
	"github.com/EugeneShtoka/yt-tui/internal/config"
	ovpkg "github.com/EugeneShtoka/yt-tui/internal/tui/overlay"
	"github.com/EugeneShtoka/yt-tui/internal/tui/playback"
)

// normalizedConfig returns a minimal but complete config (default panels +
// keybindings) usable for constructing a Root.
func normalizedConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Normalize()
	return cfg
}

func TestStartupDiagnosticsArriveAfterFirstFrame(t *testing.T) {
	r := New(t.Context(), apitest.NopBackend{}, apitest.NopBackend{}, normalizedConfig(), nil, nil, playback.YtdlpInfo{})
	r, _ = r.handleResize(80, 24)
	if r.View().Content == "" {
		t.Fatal("UI should render before diagnostics arrive")
	}
	msg := StartupDiagnosticsMsg{
		Issues: []config.ConfigIssue{{Severity: config.SeverityWarning, Message: "yt-dlp unavailable"}},
		Ytdlp:  playback.YtdlpInfo{Version: "2026.10.01"},
	}
	m, _ := r.Update(msg)
	updated := m.(Root)
	if len(updated.overlays) != 1 || !strings.Contains(updated.View().Content, "yt-dlp unavailable") {
		t.Fatal("background diagnostic issue should be displayed")
	}
}

// TestNewOpensConfigIssuesOverlay: when Load reports issues, Root surfaces them
// in a startup overlay on the first frame.
func TestNewOpensConfigIssuesOverlay(t *testing.T) {
	issues := []config.ConfigIssue{{Severity: config.SeverityWarning, Message: "x"}}
	r := New(context.Background(), apitest.NopBackend{}, apitest.NopBackend{}, normalizedConfig(), nil, issues, playback.YtdlpInfo{})
	if len(r.overlays) != 1 {
		t.Fatalf("want 1 startup overlay, got %d", len(r.overlays))
	}
	if _, ok := r.overlays[0].(ovpkg.ConfigIssues); !ok {
		t.Errorf("startup overlay is %T, want ConfigIssues", r.overlays[0])
	}
}

// TestNewNoOverlayWhenClean: a clean config opens no overlay, so nothing gets in
// the user's way on a healthy start.
func TestNewNoOverlayWhenClean(t *testing.T) {
	r := New(context.Background(), apitest.NopBackend{}, apitest.NopBackend{}, normalizedConfig(), nil, nil, playback.YtdlpInfo{})
	if len(r.overlays) != 0 {
		t.Errorf("want no startup overlay for a clean config, got %d", len(r.overlays))
	}
}
