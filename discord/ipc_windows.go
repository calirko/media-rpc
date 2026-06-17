//go:build windows

package discord

import (
	"fmt"
	"time"

	"github.com/Microsoft/go-winio"
)

func dialIPCConn() (packetConn, error) {
	// winio.DialPipe opens the pipe in overlapped (async) mode, so the
	// returned net.Conn supports deadlines and an interruptible Close.
	// os.OpenFile would give a synchronous handle whose blocking ReadFile
	// cannot be cancelled until Discord itself closes the pipe — which froze
	// the whole app while Discord was running.
	timeout := 2 * time.Second
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		c, err := winio.DialPipe(path, &timeout)
		if err == nil {
			return &ipcConn{r: c}, nil
		}
	}
	return nil, fmt.Errorf("no discord named pipe found")
}
