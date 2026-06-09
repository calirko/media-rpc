// rpc-test: diagnose Discord RPC connectivity, images, and progress bar.
// Usage:
//
//	go run ./cmd/rpc-test <app_id>              — connection + full activity test
//	go run ./cmd/rpc-test <app_id> --no-image   — skip image (isolate progress bar)
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
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rpc-test <app_id> [--no-image]")
		os.Exit(1)
	}
	appID := os.Args[1]
	noImage := len(os.Args) > 2 && os.Args[2] == "--no-image"

	fmt.Println("=== media-rpc: Discord RPC diagnostic ===")
	fmt.Printf("platform  : %s\n", runtime.GOOS)
	fmt.Printf("app_id    : %s\n\n", appID)

	var c ipc
	var err error

	// --- find IPC socket / named pipe ---
	paths := ipcPaths()
	fmt.Printf("Checking %d IPC path(s)...\n", len(paths))
	for _, p := range paths {
		conn, dialErr := dialPath(p)
		if dialErr != nil {
			fmt.Printf("  ✗ %s\n", p)
			continue
		}
		fmt.Printf("  ✓ %s — connected\n", p)
		c = &streamIPC{r: conn}
		break
	}

	if c == nil {
		fmt.Println("\nNo IPC socket found. Trying arpc WebSocket at ws://127.0.0.1:1337 ...")
		ws, _, wsErr := websocket.DefaultDialer.Dial("ws://127.0.0.1:1337/?v=1", nil)
		if wsErr != nil {
			fmt.Printf("  ✗ WebSocket: %v\n", wsErr)
			fmt.Println("\nNo Discord / arpc endpoint found. Make sure Discord is running.")
			os.Exit(1)
		}
		fmt.Println("  ✓ arpc WebSocket connected")
		c = &wsIPC{c: ws}
	}
	defer c.close()

	// --- handshake ---
	fmt.Print("\nHandshake ... ")
	if err = handshake(c, appID); err != nil {
		fmt.Printf("✗ %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ READY")

	// --- test 1: basic activity (no image, no timestamps) ---
	fmt.Println("\n[Test 1] Basic activity (details + state) ...")
	sendAndPrint(c, buildActivity(appID, false, false))

	time.Sleep(2 * time.Second)

	// --- test 2: progress bar ---
	fmt.Println("\n[Test 2] Progress bar (type=2, timestamps start+end) ...")
	sendAndPrint(c, buildActivity(appID, false, true))

	time.Sleep(2 * time.Second)

	// --- test 3: image + progress bar ---
	if !noImage {
		fmt.Println("\n[Test 3] Full activity (type=2, image, timestamps) ...")
		fmt.Println("  large_image : https://i.scdn.co/image/ab67616d0000b2731ed1e90c6c4bfc6b8cabb01a")
		fmt.Println("  small_image : https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f3b5.png")
		sendAndPrint(c, buildActivity(appID, true, true))
		fmt.Println("\n  → Check your Discord profile. You should see:")
		fmt.Println("    • \"Listening to\" header")
		fmt.Println("    • Album art square (Spotify CDN image)")
		fmt.Println("    • Music note in bottom-right corner")
		fmt.Println("    • Progress bar from 0:30 to 3:30")
	}
}

func buildActivity(appID string, withImage, withTimestamps bool) []byte {
	act := map[string]any{
		"type":    2, // Listening to
		"details": "Broken Hearts and Code",
		"state":   "DJ Wump · Test Album",
	}

	assets := map[string]any{
		"large_text":  "Test Album",
		"small_image": "https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f3b5.png",
		"small_text":  "spotify",
	}
	if withImage {
		// Public Spotify CDN image — no auth required
		assets["large_image"] = "https://i.scdn.co/image/ab67616d0000b2731ed1e90c6c4bfc6b8cabb01a"
	}
	act["assets"] = assets

	if withTimestamps {
		now := time.Now()
		act["timestamps"] = map[string]any{
			"start": now.Add(-30 * time.Second).Unix(), // 30 s into track
			"end":   now.Add(3 * time.Minute).Unix(),   // 3 min total remaining
		}
	}

	data, _ := json.Marshal(map[string]any{
		"cmd":   "SET_ACTIVITY",
		"args":  map[string]any{"pid": os.Getpid(), "activity": act},
		"nonce": fmt.Sprintf("test-%d", time.Now().UnixNano()),
	})
	return data
}

func sendAndPrint(c ipc, payload []byte) {
	fmt.Printf("  sent    : %s\n", payload)
	if err := c.send(opFrame, payload); err != nil {
		fmt.Printf("  ✗ send error: %v\n", err)
		return
	}
	_, resp, err := c.recv()
	if err != nil {
		fmt.Printf("  ✗ read error: %v\n", err)
		return
	}
	fmt.Printf("  response: %s\n", resp)
}

// ── IPC abstraction ───────────────────────────────────────────────────────────

const (
	opHandshake = 0
	opFrame     = 1
)

type ipc interface {
	send(op uint32, data []byte) error
	recv() (uint32, []byte, error)
	close()
}

type streamIPC struct{ r io.ReadWriteCloser }

func (s *streamIPC) send(op uint32, data []byte) error {
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], op)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(data)))
	_, err := s.r.Write(append(hdr, data...))
	return err
}
func (s *streamIPC) recv() (uint32, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(s.r, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(hdr[0:4])
	buf := make([]byte, binary.LittleEndian.Uint32(hdr[4:8]))
	if _, err := io.ReadFull(s.r, buf); err != nil {
		return 0, nil, err
	}
	return op, buf, nil
}
func (s *streamIPC) close() { s.r.Close() }

type wsIPC struct{ c *websocket.Conn }

func (w *wsIPC) send(op uint32, data []byte) error {
	pkt := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(pkt[0:4], op)
	binary.LittleEndian.PutUint32(pkt[4:8], uint32(len(data)))
	copy(pkt[8:], data)
	return w.c.WriteMessage(websocket.BinaryMessage, pkt)
}
func (w *wsIPC) recv() (uint32, []byte, error) {
	_, msg, err := w.c.ReadMessage()
	if err != nil || len(msg) < 8 {
		return 0, msg, err
	}
	return binary.LittleEndian.Uint32(msg[0:4]), msg[8:], nil
}
func (w *wsIPC) close() { w.c.Close() }

// ── dial helpers ──────────────────────────────────────────────────────────────

func ipcPaths() []string {
	if runtime.GOOS == "windows" {
		var p []string
		for i := 0; i < 10; i++ {
			p = append(p, fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i))
		}
		return p
	}
	var paths []string
	seen := map[string]bool{}
	for _, dir := range []string{os.Getenv("XDG_RUNTIME_DIR"), os.TempDir(), "/tmp"} {
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

func handshake(c ipc, appID string) error {
	hs, _ := json.Marshal(map[string]any{"v": 1, "client_id": appID})
	if err := c.send(opHandshake, hs); err != nil {
		return err
	}
	_, payload, err := c.recv()
	if err != nil {
		return err
	}
	var resp struct {
		Evt string `json:"evt"`
	}
	_ = json.Unmarshal(payload, &resp)
	if resp.Evt != "READY" {
		return fmt.Errorf("expected READY, got %q — raw: %s", resp.Evt, payload)
	}
	return nil
}
