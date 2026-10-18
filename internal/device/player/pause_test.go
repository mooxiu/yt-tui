package player

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPauseWithoutPlayback(t *testing.T) {
	b := newSimpleBackend(fakeDriver{})
	if err := b.TogglePause(); err == nil {
		t.Fatal("expected an error when idle")
	}
	sess := newSession(0)
	b.pauseControl.set("", sess)
	if err := b.TogglePause(); err == nil {
		t.Fatal("expected unsupported-player error")
	}
	sess.Finish()
	if err := b.TogglePause(); err == nil {
		t.Fatal("expected an error after playback ends")
	}
}

// Exercise actual audio-only mpv playback, without a display, sound device,
// network, D-Bus, or an MPRIS plugin. Read pause back from mpv, rather than
// trusting that writing an IPC command succeeded.
func TestMPVAudioPauseResume(t *testing.T) {
	path, err := exec.LookPath("mpv")
	if err != nil {
		t.Skip("mpv not installed")
	}
	dir := t.TempDir()
	source := filepath.Join(dir, "silence.wav")
	data := make([]byte, 44+16000)
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], 8000)
	binary.LittleEndian.PutUint32(data[28:], 16000)
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], 16000)
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	driver := &mpvDriver{path: path}
	b := newSimpleBackend(driver)
	args := append([]string{"--no-config", "--ao=null", "--loop-file=inf"}, driver.AudioArgs(source, "", 0)...)
	sess, err := b.exec(args)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		conn, err := net.DialTimeout("unix", filepath.Join(b.dir, "ipc"), time.Second)
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			_ = json.NewEncoder(conn).Encode(map[string]any{"command": []string{"quit"}})
			var response map[string]any
			_ = json.NewDecoder(conn).Decode(&response)
			_ = conn.Close()
		}
		select {
		case <-sess.Done():
			if _, err := os.Stat(b.dir); !os.IsNotExist(err) {
				t.Errorf("player socket directory was not removed: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Error("mpv did not exit")
		}
	})
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(b.dir, "ipc")); err == nil {
			break
		}
		select {
		case <-sess.Done():
			t.Fatal("mpv exited before its control socket was ready")
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("mpv control socket never appeared")
		}
		time.Sleep(20 * time.Millisecond)
	}
	mpris := &mprisBackend{driver: driver}
	mpris.pauseControl.set(b.dir, sess)
	for i, want := range []bool{true, false} {
		var control interface{ TogglePause() error } = b
		if i == 1 {
			control = mpris
		}
		if err := control.TogglePause(); err != nil {
			t.Fatal(err)
		}
		if got := readMPVPause(t, filepath.Join(b.dir, "ipc")); got != want {
			t.Fatalf("pause = %v, want %v", got, want)
		}
	}
}

func readMPVPause(t *testing.T, path string) bool {
	t.Helper()
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := json.NewEncoder(conn).Encode(map[string]any{
		"command": []string{"get_property", "pause"}, "request_id": 42,
	}); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(conn)
	for {
		var result struct {
			RequestID int    `json:"request_id"`
			Data      bool   `json:"data"`
			Error     string `json:"error"`
		}
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}
		if result.RequestID != 42 {
			continue
		}
		if result.Error != "success" {
			t.Fatal(result.Error)
		}
		return result.Data
	}
}
