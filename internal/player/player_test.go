package player

import (
	"bufio"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// fakeMpv is a minimal stand-in for mpv's JSON IPC server
// (https://mpv.io/manual/stable/#json-ipc), enough to exercise Player's
// request/response and event-skipping logic without spawning real mpv.
type fakeMpv struct {
	conn      net.Conn
	paused    bool
	sentEvent bool
}

func newFakeMpv(conn net.Conn) *fakeMpv {
	return &fakeMpv{conn: conn}
}

func (f *fakeMpv) writeLine(v any) {
	b, _ := json.Marshal(v)
	b = append(b, '\n')
	_, _ = f.conn.Write(b)
}

func (f *fakeMpv) serve(t *testing.T) {
	reader := bufio.NewReader(f.conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		// net.Pipe is unbuffered/synchronous, so writing an unsolicited
		// event before the client has issued its first read-triggering
		// write would deadlock both sides. Interleave it here instead,
		// ahead of the first real response: the client's call() already
		// loops reading lines until it sees a matching request_id, so this
		// still exercises the event-skipping path.
		if !f.sentEvent {
			f.sentEvent = true
			f.writeLine(map[string]any{"event": "property-change", "id": 1})
		}
		var req struct {
			Command   []any `json:"command"`
			RequestID int64 `json:"request_id"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			t.Errorf("fakeMpv: bad request JSON: %v", err)
			continue
		}
		if len(req.Command) == 0 {
			continue
		}

		switch req.Command[0] {
		case "get_property":
			prop, _ := req.Command[1].(string)
			var data any
			switch prop {
			case "time-pos":
				data = 12.5
			case "duration":
				data = 100.0
			case "pause":
				data = f.paused
			case "eof-reached":
				data = false
			}
			f.writeLine(map[string]any{"request_id": req.RequestID, "error": "success", "data": data})

		case "set_property":
			prop, _ := req.Command[1].(string)
			if prop == "pause" {
				f.paused, _ = req.Command[2].(bool)
			}
			f.writeLine(map[string]any{"request_id": req.RequestID, "error": "success"})

		case "seek":
			f.writeLine(map[string]any{"request_id": req.RequestID, "error": "success"})

		case "quit":
			f.writeLine(map[string]any{"request_id": req.RequestID, "error": "success"})
			return

		default:
			f.writeLine(map[string]any{"request_id": req.RequestID, "error": "unsupported command"})
		}
	}
}

func newTestPlayer(t *testing.T) (*Player, *fakeMpv) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	fake := newFakeMpv(serverConn)
	go fake.serve(t)

	p := newPlayer(nil, clientConn, "", "test stream")
	t.Cleanup(func() { _ = clientConn.Close() })
	return p, fake
}

func TestPlayerStatusAndControls(t *testing.T) {
	p, _ := newTestPlayer(t)

	status := p.Status()
	if status.Position != 12.5 || status.Duration != 100.0 {
		t.Errorf("Status() = %+v, want Position=12.5 Duration=100", status)
	}
	if status.Paused {
		t.Error("Status().Paused = true, want false initially")
	}

	if err := p.TogglePause(); err != nil {
		t.Fatalf("TogglePause: %v", err)
	}
	if got := p.Status(); !got.Paused {
		t.Errorf("after TogglePause, Status().Paused = false, want true")
	}

	if err := p.Seek(10); err != nil {
		t.Fatalf("Seek: %v", err)
	}
}

func TestPlayerCallIgnoresUnsolicitedEvents(t *testing.T) {
	// newTestPlayer's fake server writes a bogus "event" line before ever
	// seeing a request; the first real call must still resolve correctly
	// rather than getting confused by it.
	p, _ := newTestPlayer(t)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if got := p.getFloat("duration"); got != 100.0 {
			t.Errorf("getFloat(duration) = %v, want 100", got)
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("call() hung, likely mis-parsed the unsolicited event")
	}
}
