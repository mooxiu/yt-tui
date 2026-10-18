package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// pauseControl targets the exact mpv instance launched by this backend. The
// private directory prevents other users from issuing commands to the player.
type pauseControl struct {
	mu   sync.Mutex
	dir  string
	sess *Session
}

func preparePauseControl(driver Driver, args []string) ([]string, string, error) {
	if _, ok := driver.(*mpvDriver); !ok {
		return args, "", nil
	}
	dir, err := os.MkdirTemp("", "yt-tui-mpv-")
	if err != nil {
		return nil, "", fmt.Errorf("player control: %w", err)
	}
	return append([]string{"--input-ipc-server=" + filepath.Join(dir, "ipc")}, args...), dir, nil
}

func cleanupPauseControl(dir string) {
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

func (p *pauseControl) set(dir string, sess *Session) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dir, p.sess = dir, sess
}

func (p *pauseControl) TogglePause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sess == nil {
		return errors.New("nothing is playing")
	}
	select {
	case <-p.sess.Done():
		return errors.New("nothing is playing")
	default:
	}
	if p.dir == "" {
		return errors.New("pause/resume requires mpv or the mpris backend")
	}
	conn, err := net.DialTimeout("unix", filepath.Join(p.dir, "ipc"), 2*time.Second)
	if err != nil {
		return fmt.Errorf("player control unavailable (the player may still be starting): %w", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		return fmt.Errorf("player control: %w", err)
	}
	if err := json.NewEncoder(conn).Encode(map[string]any{
		"command": []string{"cycle", "pause"}, "request_id": 1,
	}); err != nil {
		return fmt.Errorf("player control: %w", err)
	}
	decoder := json.NewDecoder(conn)
	for {
		var response struct {
			RequestID int    `json:"request_id"`
			Error     string `json:"error"`
		}
		if err := decoder.Decode(&response); err != nil {
			return fmt.Errorf("player control response: %w", err)
		}
		// mpv can interleave unsolicited events with command responses.
		if response.RequestID != 1 {
			continue
		}
		if response.Error != "success" {
			return fmt.Errorf("mpv: %s", response.Error)
		}
		return nil
	}
}

// TogglePause uses native IPC for mpv, including when no MPRIS plugin is
// installed. Other MPRIS players are resolved by PID to avoid controlling an
// unrelated player instance.
func (b *mprisBackend) TogglePause() error {
	if _, ok := b.driver.(*mpvDriver); ok {
		return b.pauseControl.TogglePause()
	}
	active, ok := b.Active()
	if !ok {
		return errors.New("nothing is playing")
	}
	name, err := b.resolveBusName(active.PID)
	if err != nil {
		return fmt.Errorf("player control unavailable: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := b.conn.Object(name, "/org/mpris/MediaPlayer2").CallWithContext(ctx,
		"org.mpris.MediaPlayer2.Player.PlayPause", 0).Err; err != nil {
		return fmt.Errorf("MPRIS pause/resume: %w", err)
	}
	return nil
}
