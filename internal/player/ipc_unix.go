//go:build !windows

package player

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// ipcAddress returns a per-process-unique Unix domain socket path for mpv's
// --input-ipc-server.
func ipcAddress() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("nuvio-mpv-%d.sock", os.Getpid()))
}

func dialIPC(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}

func cleanupIPC(addr string) {
	_ = os.Remove(addr)
}
