//go:build linux

package discord

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
)

func dialIPCConn() (packetConn, error) {
	for _, path := range ipcSocketPaths() {
		c, err := net.Dial("unix", path)
		if err == nil {
			return &ipcConn{r: c}, nil
		}
	}
	return nil, fmt.Errorf("no discord IPC socket found")
}

func ipcSocketPaths() []string {
	var paths []string
	dirs := []string{
		os.Getenv("XDG_RUNTIME_DIR"),
		os.TempDir(),
		"/tmp",
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		if dir == "" || seen[dir] {
			continue
		}
		seen[dir] = true
		for i := 0; i < 10; i++ {
			paths = append(paths, filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i)))
		}
	}
	return paths
}
