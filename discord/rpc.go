package discord

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"media-rpc/config"
	"media-rpc/media"
)

const (
	opHandshake = 0
	opFrame     = 1
)

type RPC struct {
	mu          sync.Mutex
	appID       string
	c           packetConn
	pid         int
	nonce       atomic.Int64
	playerIcons map[string]string
	smallIcon   string
}

func New(cfg *config.Config) *RPC {
	icons := cfg.PlayerIcons
	if icons == nil {
		icons = map[string]string{}
	}
	return &RPC{
		appID:       cfg.AppID,
		pid:         os.Getpid(),
		playerIcons: icons,
		smallIcon:   cfg.SmallIcon,
	}
}

func (r *RPC) SetActivity(info media.Info) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.c == nil {
		r.c = r.connect()
		if r.c == nil {
			return
		}
	}
	payload := r.buildSetActivity(&info)
	if err := r.c.sendPacket(opFrame, payload); err != nil {
		log.Println("discord: send failed, reconnecting next tick:", err)
		r.c.close()
		r.c = nil
	}
}

func (r *RPC) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.c == nil {
		return
	}
	payload := r.buildClearActivity()
	if err := r.c.sendPacket(opFrame, payload); err != nil {
		r.c.close()
		r.c = nil
	}
}

func (r *RPC) connect() packetConn {
	// try standard Discord IPC
	if c, err := dialIPCConn(); err == nil {
		if err := handshake(c, r.appID); err == nil {
			go drain(c, &r.mu, &r.c)
			return c
		}
		c.close()
	}
	// try arpc (Vencord / arrpc WebSocket bridge)
	if c := dialArpc(r.appID); c != nil {
		go drain(c, &r.mu, &r.c)
		return c
	}
	return nil
}

func dialArpc(appID string) packetConn {
	ws, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:1337/?v=1", nil)
	if err != nil {
		return nil
	}
	c := &wsConn{c: ws}
	if err := handshake(c, appID); err != nil {
		c.close()
		return nil
	}
	return c
}

func handshake(c packetConn, appID string) error {
	hs, _ := json.Marshal(map[string]any{"v": 1, "client_id": appID})
	if err := c.sendPacket(opHandshake, hs); err != nil {
		return err
	}
	_, payload, err := c.recvPacket()
	if err != nil {
		return err
	}
	var resp struct {
		Evt string `json:"evt"`
	}
	if err := json.Unmarshal(payload, &resp); err != nil {
		return err
	}
	if resp.Evt != "READY" {
		return fmt.Errorf("expected READY, got %q", resp.Evt)
	}
	return nil
}

// drain continuously reads and discards Discord responses so the socket buffer
// never fills. Clears the connection on error so the next send triggers reconnect.
func drain(c packetConn, mu *sync.Mutex, slot *packetConn) {
	for {
		if _, _, err := c.recvPacket(); err != nil {
			mu.Lock()
			if *slot == c {
				*slot = nil
			}
			mu.Unlock()
			return
		}
	}
}

type rpcFrame struct {
	Cmd   string `json:"cmd"`
	Args  any    `json:"args"`
	Nonce string `json:"nonce"`
}

type activityPayload struct {
	PID      int      `json:"pid"`
	Activity *activity `json:"activity"`
}

type activity struct {
	Details    string      `json:"details,omitempty"`
	State      string      `json:"state,omitempty"`
	Assets     *imgAssets  `json:"assets,omitempty"`
	Timestamps *timestamps `json:"timestamps,omitempty"`
}

type imgAssets struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
	SmallImage string `json:"small_image,omitempty"`
	SmallText  string `json:"small_text,omitempty"`
}

type timestamps struct {
	Start int64 `json:"start,omitempty"`
	End   int64 `json:"end,omitempty"`
}

func (r *RPC) buildSetActivity(info *media.Info) []byte {
	act := activity{
		Details: info.Title,
		State:   stateLabel(info),
	}

	if !info.StartedAt.IsZero() {
		ts := &timestamps{Start: info.StartedAt.Unix()}
		end := info.StartedAt.Add(info.Duration)
		// Only set end if duration is known and position hasn't overshot it (seek edge case)
		if info.Duration > 0 && end.After(info.StartedAt) {
			ts.End = end.Unix()
		}
		act.Timestamps = ts
	}

	smallImg, smallTip := r.resolveSmallIcon(info.Player)
	if strings.HasPrefix(info.ArtURL, "http") {
		act.Assets = &imgAssets{
			LargeImage: info.ArtURL,
			LargeText:  info.Album,
			SmallImage: smallImg,
			SmallText:  smallTip,
		}
	} else if info.Album != "" || smallImg != "" {
		act.Assets = &imgAssets{
			LargeText:  info.Album,
			SmallImage: smallImg,
			SmallText:  smallTip,
		}
	}

	data, _ := json.Marshal(rpcFrame{
		Cmd:   "SET_ACTIVITY",
		Args:  activityPayload{PID: r.pid, Activity: &act},
		Nonce: fmt.Sprintf("%d", r.nonce.Add(1)),
	})
	return data
}

// resolveSmallIcon returns the icon URL/key and tooltip for the small overlay image.
// Priority: player_icons config → smallIcon default → empty (no small image).
func (r *RPC) resolveSmallIcon(player string) (url, tip string) {
	player = strings.ToLower(player)
	if u, ok := r.playerIcons[player]; ok && u != "" {
		return u, capitalize(player)
	}
	return r.smallIcon, capitalize(player)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// stateLabel combines artist and album into a single Discord state line.
func stateLabel(info *media.Info) string {
	switch {
	case info.Artist != "" && info.Album != "":
		return info.Artist + " · " + info.Album
	case info.Artist != "":
		return info.Artist
	default:
		return info.Album
	}
}

func (r *RPC) buildClearActivity() []byte {
	data, _ := json.Marshal(rpcFrame{
		Cmd:   "SET_ACTIVITY",
		Args:  activityPayload{PID: r.pid, Activity: nil},
		Nonce: fmt.Sprintf("%d", r.nonce.Add(1)),
	})
	return data
}
