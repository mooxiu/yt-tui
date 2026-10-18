package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/EugeneShtoka/yt-tui/internal/config"
	tuipkg "github.com/EugeneShtoka/yt-tui/internal/tui"
	"github.com/EugeneShtoka/yt-tui/internal/tui/keymap"
	ovpkg "github.com/EugeneShtoka/yt-tui/internal/tui/overlay"
)

func TestPauseKeyRouting(t *testing.T) {
	for _, tc := range []struct {
		name         string
		tabInput     bool
		overlay      bool
		overlayInput bool
	}{
		{name: "active tab"},
		{name: "tab text input", tabInput: true},
		{name: "focused overlay", overlay: true},
		{name: "overlay text input", overlay: true, overlayInput: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRootNoOverlay(fakeTab{intercepts: tc.tabInput})
			r.keys = keymap.Build(config.KeyBindings{TogglePause: "space"})
			if tc.overlay {
				r.overlays = []ovpkg.Overlay{fakeOverlay{hasFocus: true, intercepts: tc.overlayInput}}
			}
			got, cmd := r.handleKey(tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
			if tc.tabInput || tc.overlay {
				if cmd != nil {
					t.Fatal("space in a text input should not control playback")
				}
				if tc.overlay {
					if len(got.overlays[0].(fakeOverlay).received) != 1 {
						t.Fatal("overlay input lost space")
					}
				} else if len(got.activeTab().(fakeTab).received) != 1 {
					t.Fatal("tab input lost space")
				}
				return
			}
			if cmd == nil {
				t.Fatal("space did not control playback")
			}
			msg, ok := cmd().(tuipkg.StatusMsg)
			if !ok || !msg.IsErr || msg.Text != "nothing is playing" {
				t.Fatalf("expected idle playback status, got %#v", msg)
			}
			if len(got.activeTab().(fakeTab).received) != 0 {
				t.Fatal("pause key reached tab")
			}
		})
	}
}
