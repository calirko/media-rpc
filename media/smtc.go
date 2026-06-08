//go:build windows

package media

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type smtcSource struct{}

func newSMTCSource() Source { return &smtcSource{} }

// smtcScript reads all active SMTC sessions via WinRT through PowerShell.
// Each line: AppID|Title|Artist|Album|Status|PositionSec|DurationSec
const smtcScript = `
try {
    $null = [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager, Windows.Media.Control, ContentType=WindowsRuntime]
    $mgr = [Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager]::RequestAsync().GetAwaiter().GetResult()
    foreach ($s in $mgr.GetSessions()) {
        $p  = $s.TryGetMediaPropertiesAsync().GetAwaiter().GetResult()
        $pb = $s.GetPlaybackInfo().PlaybackStatus
        $tl = $s.GetTimelineProperties()
        $pos = $tl.Position.TotalSeconds
        $dur = ($tl.EndTime - $tl.StartTime).TotalSeconds
        Write-Output "$($s.SourceAppUserModelId)|$($p.Title)|$($p.Artist)|$($p.AlbumTitle)|$pb|$pos|$dur"
    }
} catch {}
`

func (s *smtcSource) Players() ([]Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx,
		"powershell", "-NoProfile", "-NonInteractive", "-Command", smtcScript,
	).Output()
	if err != nil {
		return nil, err
	}

	var players []Info
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 7)
		if len(parts) < 5 {
			continue
		}

		info := Info{
			Player:  cleanAppID(parts[0]),
			Title:   parts[1],
			Artist:  parts[2],
			Album:   parts[3],
			Playing: strings.TrimSpace(parts[4]) == "Playing",
		}

		if len(parts) >= 7 {
			posSec, _ := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
			durSec, _ := strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
			if posSec >= 0 {
				info.StartedAt = time.Now().Add(-time.Duration(posSec * float64(time.Second)))
			}
			if durSec > 0 {
				info.Duration = time.Duration(durSec * float64(time.Second))
			}
		}
		if info.StartedAt.IsZero() {
			info.StartedAt = time.Now()
		}

		players = append(players, info)
	}
	return players, nil
}

// cleanAppID turns "Spotify.exe" or "com.spotify.client!App" into "spotify".
func cleanAppID(id string) string {
	id = strings.ToLower(id)
	id = strings.TrimSuffix(id, ".exe")
	if i := strings.LastIndexAny(id, "!."); i != -1 {
		id = id[i+1:]
	}
	return id
}
