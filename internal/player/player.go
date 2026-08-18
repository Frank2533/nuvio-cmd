// Package player launches and remote-controls the user's installed mpv
// binary over its JSON IPC socket (https://mpv.io/manual/stable/#json-ipc).
// Nuvio CMD never embeds libmpv or renders video itself: mpv owns its own
// window, and this package is purely a control channel (play/pause/seek/
// position) plus a way to detect when playback ends.
package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"
)

// Status is a point-in-time snapshot of mpv's playback state.
type Status struct {
	Position   float64
	Duration   float64
	Paused     bool
	EOFReached bool
}

// Player is one running, IPC-controlled mpv process.
type Player struct {
	Title string

	cmd      *exec.Cmd
	conn     net.Conn
	reader   *bufio.Reader
	sockAddr string

	mu    sync.Mutex
	reqID int64
}

// Start launches mpv playing streamURL and connects to its IPC socket. mpv
// must already be installed and on PATH; Nuvio CMD does not bundle it.
func Start(streamURL, title string) (*Player, error) {
	mpvPath, err := exec.LookPath("mpv")
	if err != nil {
		return nil, fmt.Errorf("mpv not found on PATH — install mpv to enable playback: %w", err)
	}

	addr := ipcAddress()
	args := []string{
		"--force-window=yes",
		"--input-ipc-server=" + addr,
		"--title=" + title,
		streamURL,
	}

	cmd := exec.Command(mpvPath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start mpv: %w", err)
	}

	conn, err := dialWithRetry(addr, 5*time.Second)
	if err != nil {
		_ = cmd.Process.Kill()
		return nil, fmt.Errorf("connect to mpv ipc socket: %w", err)
	}

	return newPlayer(cmd, conn, addr, title), nil
}

// newPlayer wires up a Player around an already-connected IPC conn. Split
// out from Start so tests can exercise the IPC protocol against a fake
// server without spawning a real mpv process.
func newPlayer(cmd *exec.Cmd, conn net.Conn, sockAddr, title string) *Player {
	return &Player{
		Title:    title,
		cmd:      cmd,
		conn:     conn,
		reader:   bufio.NewReader(conn),
		sockAddr: sockAddr,
	}
}

func dialWithRetry(addr string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := dialIPC(addr)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	return nil, lastErr
}

// call sends an mpv IPC command and waits for the response matching its
// request_id, discarding any event notifications in between.
func (p *Player) call(cmd []any) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.reqID++
	id := p.reqID

	payload, err := json.Marshal(map[string]any{"command": cmd, "request_id": id})
	if err != nil {
		return nil, err
	}
	if _, err := p.conn.Write(append(payload, '\n')); err != nil {
		return nil, err
	}

	for {
		line, err := p.reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}

		var resp struct {
			RequestID int64           `json:"request_id"`
			Error     string          `json:"error"`
			Data      json.RawMessage `json:"data"`
			Event     string          `json:"event"`
		}
		if err := json.Unmarshal(line, &resp); err != nil {
			continue // not valid JSON we care about
		}
		if resp.Event != "" || resp.RequestID != id {
			continue // an event notification or a stale response
		}
		if resp.Error != "" && resp.Error != "success" {
			return nil, fmt.Errorf("mpv: %s", resp.Error)
		}
		return resp.Data, nil
	}
}

func (p *Player) getFloat(prop string) float64 {
	data, err := p.call([]any{"get_property", prop})
	if err != nil || data == nil {
		return 0
	}
	var f float64
	_ = json.Unmarshal(data, &f)
	return f
}

func (p *Player) getBool(prop string) bool {
	data, err := p.call([]any{"get_property", prop})
	if err != nil || data == nil {
		return false
	}
	var b bool
	_ = json.Unmarshal(data, &b)
	return b
}

// Status polls mpv for the current playback position/duration/pause state.
// Individual property errors (e.g. "time-pos" not yet available right after
// start) are swallowed to keep a polling UI resilient.
func (p *Player) Status() Status {
	return Status{
		Position:   p.getFloat("time-pos"),
		Duration:   p.getFloat("duration"),
		Paused:     p.getBool("pause"),
		EOFReached: p.getBool("eof-reached"),
	}
}

// TogglePause flips mpv's pause state.
func (p *Player) TogglePause() error {
	_, err := p.call([]any{"set_property", "pause", !p.getBool("pause")})
	return err
}

// Seek moves playback by deltaSeconds, relative to the current position.
func (p *Player) Seek(deltaSeconds float64) error {
	_, err := p.call([]any{"seek", deltaSeconds, "relative"})
	return err
}

// Wait blocks until the mpv process exits (user closed the window, or
// playback finished and mpv wasn't started with --keep-open).
func (p *Player) Wait() error {
	return p.cmd.Wait()
}

// Stop asks mpv to quit and releases IPC resources.
func (p *Player) Stop() {
	_, _ = p.call([]any{"quit"})
	_ = p.conn.Close()
	cleanupIPC(p.sockAddr)
}
