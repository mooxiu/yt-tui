package playback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EugeneShtoka/yt-tui/internal/device/player"
	tuipkg "github.com/EugeneShtoka/yt-tui/internal/tui"
)

type pausePlayer struct {
	calls int
	err   error
}

func (p *pausePlayer) Launch(string, string, string, time.Duration) (*player.Session, error) {
	return nil, nil
}
func (p *pausePlayer) LaunchAudio(string, string, string, time.Duration) (*player.Session, error) {
	return nil, nil
}
func (p *pausePlayer) Active() (player.ActivePlayback, bool) { return player.ActivePlayback{}, false }
func (p *pausePlayer) Close()                                {}
func (p *pausePlayer) TogglePause() error                    { p.calls++; return p.err }

func TestTogglePauseCmd(t *testing.T) {
	for _, fail := range []bool{false, true} {
		p := &pausePlayer{}
		if fail {
			p.err = errors.New("nothing is playing")
		}
		c := New(context.Background(), &fakeBackend{}, p, YtdlpInfo{})
		msg, ok := c.TogglePauseCmd()().(tuipkg.StatusMsg)
		if !ok || msg.IsErr != fail || p.calls != 1 {
			t.Fatalf("pause command: calls=%d, status=%#v", p.calls, msg)
		}
	}
}
