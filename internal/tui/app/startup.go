package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/EugeneShtoka/yt-tui/internal/config"
	"github.com/EugeneShtoka/yt-tui/internal/tui/playback"
)

// StartupDiagnosticsMsg delivers environment checks after the UI has started.
type StartupDiagnosticsMsg struct {
	Issues []config.ConfigIssue
	Ytdlp  playback.YtdlpInfo
}

// WithStartupDiagnostics schedules checks as a command, keeping subprocesses
// and remote health requests off the path to the first frame.
func (r Root) WithStartupDiagnostics(cmd tea.Cmd) Root {
	r.startupDiagnostics = cmd
	return r
}
