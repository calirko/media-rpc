//go:build linux

package media

import (
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

type mprisSource struct{}

func newMPRISSource() Source { return &mprisSource{} }

func (s *mprisSource) Players() ([]Info, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return nil, err
	}

	var names []string
	if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
		return nil, err
	}

	var players []Info
	for _, name := range names {
		if !strings.HasPrefix(name, "org.mpris.MediaPlayer2.") {
			continue
		}
		id := strings.TrimPrefix(name, "org.mpris.MediaPlayer2.")
		// strip instance suffix like ".instance12345"
		if i := strings.Index(id, "."); i != -1 {
			id = id[:i]
		}
		info, err := queryMPRIS(conn, name, strings.ToLower(id))
		if err != nil {
			continue
		}
		players = append(players, info)
	}
	return players, nil
}

func queryMPRIS(conn *dbus.Conn, service, id string) (Info, error) {
	obj := conn.Object(service, "/org/mpris/MediaPlayer2")

	statusVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.PlaybackStatus")
	if err != nil {
		return Info{}, err
	}
	status, _ := statusVar.Value().(string)

	metaVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Metadata")
	if err != nil {
		return Info{}, err
	}

	info := Info{
		Player:    id,
		Playing:   status == "Playing",
		StartedAt: time.Now(),
	}

	if meta, ok := metaVar.Value().(map[string]dbus.Variant); ok {
		if v, ok := meta["xesam:title"]; ok {
			info.Title, _ = v.Value().(string)
		}
		if v, ok := meta["xesam:artist"]; ok {
			if artists, ok := v.Value().([]string); ok && len(artists) > 0 {
				info.Artist = strings.Join(artists, ", ")
			}
		}
		if v, ok := meta["xesam:album"]; ok {
			info.Album, _ = v.Value().(string)
		}
		if v, ok := meta["mpris:artUrl"]; ok {
			rawArt, _ := v.Value().(string)
			info.ArtURL = rawArt // resolved below
		}
		if v, ok := meta["xesam:url"]; ok {
			info.TrackURL, _ = v.Value().(string)
		}
		if v, ok := meta["mpris:length"]; ok {
			// length is in microseconds; handle both int64 and uint64
			switch l := v.Value().(type) {
			case int64:
				if l > 0 {
					info.Duration = time.Duration(l) * time.Microsecond
				}
			case uint64:
				if l > 0 {
					info.Duration = time.Duration(l) * time.Microsecond
				}
			}
		}
	}

	info.ArtURL = resolveArtURL(info.ArtURL, info.TrackURL)

	// Use player position for an accurate start timestamp
	posVar, err := obj.GetProperty("org.mpris.MediaPlayer2.Player.Position")
	if err == nil {
		if pos, ok := posVar.Value().(int64); ok && pos >= 0 {
			info.StartedAt = time.Now().Add(-time.Duration(pos) * time.Microsecond)
		}
	}

	return info, nil
}
