// rpc-test: diagnose Discord RPC connectivity.
// Run with: go run ./cmd/rpc-test
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gorilla/websocket"
)

func main() {
	appID := "1234567890" // placeholder — handshake only needs a non-empty string
	if len(os.Args) > 1 {
		appID = os.Args[1]
	}

	fmt.Println("=== media-rpc: Discord RPC diagnostic ===")
	fmt.Printf("platform : %s\n", runtime.GOOS)
	fmt.Printf("app_id   : %s\n\n", appID)

	// --- IPC sockets / named pipes ---
	paths := ipcPaths()
	fmt.Printf("Checking %d IPC path(s)...\n", len(paths))
	var connected io.ReadWriteCloser
	var connPath string
	for _, p := range paths {
		conn, err := dialPath(p)
		if err != nil {
			fmt.Printf("  ✗ %s — %v\n", p, err)
			continue
		}
		fmt.Printf("  ✓ %s — connected\n", p)
		connected = conn
		connPath = p
		break
	}

	if connected != nil {
		fmt.Printf("\nHandshake on %s ...\n", connPath)
		if err := doHandshake(connected, appID); err != nil {
			fmt.Printf("  ✗ handshake failed: %v\n", err)
		} else {
			fmt.Println("  ✓ READY — Discord RPC is reachable via IPC")
			sendTestActivity(connected)
		}
		connected.Close()
		return
	}

	// --- arpc / arrpc WebSocket fallback ---
	fmt.Println("\nNo IPC socket found. Trying arpc WebSocket at ws://127.0.0.1:1337 ...")
	ws, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:1337/?v=1", nil)
	if err != nil {
		fmt.Printf("  ✗ WebSocket failed: %v\n", err)
		fmt.Println("\nConclusion: no Discord / arpc RPC endpoint found.")
		fmt.Println("  • Make sure Discord (or Vencord with arrpc) is running.")
		fmt.Println("  • On Linux, check $XDG_RUNTIME_DIR for discord-ipc-0 through discord-ipc-9.")
		os.Exit(1)
	}
	fmt.Println("  ✓ WebSocket connected — arpc is running")
	wc := &wsConn{c: ws}
	if err := doHandshakeConn(wc, appID); err != nil {
		fmt.Printf("  ✗ handshake failed: %v\n", err)
	} else {
		fmt.Println("  ✓ READY — Discord RPC is reachable via arpc WebSocket")
		sendTestActivityConn(wc)
	}
	ws.Close()
}

// --- IPC path helpers ---

func ipcPaths() []string {
	if runtime.GOOS == "windows" {
		var paths []string
		for i := 0; i < 10; i++ {
			paths = append(paths, fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i))
		}
		return paths
	}
	// Linux / macOS
	var paths []string
	dirs := []string{os.Getenv("XDG_RUNTIME_DIR"), os.TempDir(), "/tmp"}
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

func dialPath(path string) (io.ReadWriteCloser, error) {
	if runtime.GOOS == "windows" {
		return os.OpenFile(path, os.O_RDWR, 0)
	}
	return net.Dial("unix", path)
}

// --- IPC packet protocol ---

func writePacket(w io.Writer, op uint32, data []byte) error {
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], op)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(data)))
	_, err := w.Write(append(hdr, data...))
	return err
}

func readPacket(r io.Reader) (uint32, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(hdr[0:4])
	n := binary.LittleEndian.Uint32(hdr[4:8])
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return op, payload, nil
}

func doHandshake(rw io.ReadWriter, appID string) error {
	hs, _ := json.Marshal(map[string]any{"v": 1, "client_id": appID})
	if err := writePacket(rw, 0, hs); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	_, payload, err := readPacket(rw)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	fmt.Printf("  raw response: %s\n", payload)
	var resp struct {
		Evt string `json:"evt"`
	}
	_ = json.Unmarshal(payload, &resp)
	if resp.Evt != "READY" {
		return fmt.Errorf("expected evt=READY, got %q", resp.Evt)
	}
	return nil
}

func sendTestActivity(rw io.ReadWriter) {
	fmt.Println("\nSending test activity (visible in Discord for ~5 s) ...")
	cmd, _ := json.Marshal(map[string]any{
		"cmd": "SET_ACTIVITY",
		"args": map[string]any{
			"pid": os.Getpid(),
			"activity": map[string]any{
				"details": "media-rpc test",
				"state":   "RPC connection OK",
			},
		},
		"nonce": "test-1",
	})
	if err := writePacket(rw, 1, cmd); err != nil {
		fmt.Printf("  ✗ send failed: %v\n", err)
		return
	}
	_, payload, err := readPacket(rw)
	if err != nil {
		fmt.Printf("  ✗ response read failed: %v\n", err)
		return
	}
	fmt.Printf("  response: %s\n", payload)
	fmt.Println("  ✓ activity set — check your Discord profile")
}

// --- WebSocket wrapper (same binary framing as IPC) ---

type wsConn struct{ c *websocket.Conn }

func (w *wsConn) sendPacket(op uint32, data []byte) error {
	pkt := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(pkt[0:4], op)
	binary.LittleEndian.PutUint32(pkt[4:8], uint32(len(data)))
	copy(pkt[8:], data)
	return w.c.WriteMessage(websocket.BinaryMessage, pkt)
}

func (w *wsConn) recvPacket() (uint32, []byte, error) {
	_, msg, err := w.c.ReadMessage()
	if err != nil || len(msg) < 8 {
		return 0, msg, err
	}
	op := binary.LittleEndian.Uint32(msg[0:4])
	return op, msg[8:], nil
}

func doHandshakeConn(wc *wsConn, appID string) error {
	hs, _ := json.Marshal(map[string]any{"v": 1, "client_id": appID})
	if err := wc.sendPacket(0, hs); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	_, payload, err := wc.recvPacket()
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	fmt.Printf("  raw response: %s\n", payload)
	var resp struct {
		Evt string `json:"evt"`
	}
	_ = json.Unmarshal(payload, &resp)
	if resp.Evt != "READY" {
		return fmt.Errorf("expected evt=READY, got %q", resp.Evt)
	}
	return nil
}

func sendTestActivityConn(wc *wsConn) {
	fmt.Println("\nSending test activity (visible in Discord for ~5 s) ...")
	cmd, _ := json.Marshal(map[string]any{
		"cmd": "SET_ACTIVITY",
		"args": map[string]any{
			"pid": os.Getpid(),
			"activity": map[string]any{
				"details": "media-rpc test",
				"state":   "arpc connection OK",
			},
		},
		"nonce": "test-1",
	})
	if err := wc.sendPacket(1, cmd); err != nil {
		fmt.Printf("  ✗ send failed: %v\n", err)
		return
	}
	_, payload, err := wc.recvPacket()
	if err != nil {
		fmt.Printf("  ✗ response read failed: %v\n", err)
		return
	}
	fmt.Printf("  response: %s\n", payload)
	fmt.Println("  ✓ activity set — check your Discord profile")
}
