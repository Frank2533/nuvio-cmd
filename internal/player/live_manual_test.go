//go:build manual

package player

import (
	"testing"
	"time"
)

func TestLiveMpvPlayback(t *testing.T) {
	p, err := Start("av://lavfi:testsrc=size=320x240:rate=5", "nuvio live smoke test")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	var status Status
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		status = p.Status()
		if status.Position > 0 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Logf("status after startup: %+v", status)
	if status.Position <= 0 {
		t.Fatal("time-pos never advanced past 0 — playback did not start")
	}

	if err := p.TogglePause(); err != nil {
		t.Fatalf("TogglePause: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if got := p.Status(); !got.Paused {
		t.Fatalf("after TogglePause, Paused = false, want true (status=%+v)", got)
	}
	t.Logf("paused ok")

	if err := p.TogglePause(); err != nil {
		t.Fatalf("TogglePause (resume): %v", err)
	}
	if err := p.Seek(5); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	t.Logf("resumed and seeked ok")
}
