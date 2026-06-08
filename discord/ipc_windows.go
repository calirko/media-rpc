//go:build windows

package discord

import (
	"fmt"
	"os"
)

func dialIPCConn() (packetConn, error) {
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			return &ipcConn{r: f}, nil
		}
	}
	return nil, fmt.Errorf("no discord named pipe found")
}
