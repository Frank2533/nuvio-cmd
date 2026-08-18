//go:build windows

package player

import (
	"fmt"
	"net"
	"os"

	winio "github.com/Microsoft/go-winio"
)

// ipcAddress returns a per-process-unique named pipe path for mpv's
// --input-ipc-server on Windows.
func ipcAddress() string {
	return fmt.Sprintf(`\\.\pipe\nuvio-mpv-%d`, os.Getpid())
}

func dialIPC(addr string) (net.Conn, error) {
	return winio.DialPipe(addr, nil)
}

func cleanupIPC(addr string) {
	// Named pipes are cleaned up by the OS once both ends close; nothing to
	// remove from disk like the Unix socket case.
}
