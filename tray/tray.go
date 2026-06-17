package tray

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"fyne.io/systray"
	"media-rpc/config"
	"media-rpc/discord"
	"media-rpc/media"
)

type Tray struct {
	mu      sync.RWMutex
	cfg     *config.Config
	cfgPath string
	mgr     *media.Manager
	rpc     *discord.RPC
	enabled bool

	mNowPlaying *systray.MenuItem
	mEnabled    *systray.MenuItem
	mSource     *systray.MenuItem
	mBlacklist  *systray.MenuItem
}

func New(cfg *config.Config, cfgPath string, mgr *media.Manager, rpc *discord.RPC) *Tray {
	return &Tray{
		cfg:     cfg,
		cfgPath: cfgPath,
		mgr:     mgr,
		rpc:     rpc,
		enabled: true,
	}
}

func (t *Tray) IsEnabled() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.enabled
}

func (t *Tray) Update(info *media.Info) {
	if t.mNowPlaying == nil {
		return
	}
	if info == nil {
		t.mNowPlaying.SetTitle("No media playing")
		t.mBlacklist.SetTitle("Blacklist Current Source")
	} else {
		label := info.Title
		if info.Artist != "" {
			label = info.Artist + " — " + info.Title
		}
		if len([]rune(label)) > 50 {
			label = string([]rune(label)[:47]) + "..."
		}
		t.mNowPlaying.SetTitle(label)
		t.mBlacklist.SetTitle("Blacklist: " + info.Player)
	}

	forced := t.mgr.Forced()
	if forced == "" {
		t.mSource.SetTitle("Source: Auto")
	} else {
		t.mSource.SetTitle("Source: " + forced)
	}
}

func (t *Tray) Run() {
	systray.Run(t.onReady, func() {})
}

func (t *Tray) onReady() {
	systray.SetIcon(makeIcon())
	systray.SetTooltip("media-rpc")

	t.mNowPlaying = systray.AddMenuItem("No media playing", "")
	t.mNowPlaying.Disable()

	systray.AddSeparator()

	t.mEnabled = systray.AddMenuItem("● RPC Active", "Toggle Discord RPC")

	systray.AddSeparator()

	t.mSource = systray.AddMenuItem("Source: Auto", "Click to cycle through active players")
	t.mBlacklist = systray.AddMenuItem("Blacklist Current Source", "Add current player to blacklist")

	systray.AddSeparator()

	mOpenConfig := systray.AddMenuItem("Open Config", "Open config.json in default editor")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit media-rpc")

	go t.handleEvents(mOpenConfig, mQuit)
}

func (t *Tray) handleEvents(mOpenConfig, mQuit *systray.MenuItem) {
	for {
		select {
		case <-t.mEnabled.ClickedCh:
			t.toggleEnabled()
		case <-t.mSource.ClickedCh:
			t.cycleSource()
		case <-t.mBlacklist.ClickedCh:
			t.blacklistCurrent()
		case <-mOpenConfig.ClickedCh:
			openFile(t.cfgPath)
		case <-mQuit.ClickedCh:
			t.rpc.Clear()
			systray.Quit()
			os.Exit(0)
		}
	}
}

func (t *Tray) toggleEnabled() {
	t.mu.Lock()
	t.enabled = !t.enabled
	enabled := t.enabled
	t.mu.Unlock()

	if enabled {
		t.mEnabled.SetTitle("● RPC Active")
	} else {
		t.mEnabled.SetTitle("○ RPC Inactive")
		t.rpc.Clear()
	}
}

func (t *Tray) cycleSource() {
	all := t.mgr.Players()
	current := t.mgr.Forced()

	// build options: "" (auto) + each non-blacklisted playing player
	options := []string{""}
	for _, p := range all {
		if p.Playing && !t.cfg.IsBlacklisted(p.Player) {
			options = append(options, p.Player)
		}
	}

	next := ""
	for i, opt := range options {
		if opt == current {
			if i+1 < len(options) {
				next = options[i+1]
			}
			break
		}
	}

	t.mgr.SetForced(next)
	if next == "" {
		t.mSource.SetTitle("Source: Auto")
	} else {
		t.mSource.SetTitle("Source: " + next)
	}
}

func (t *Tray) blacklistCurrent() {
	info := t.mgr.Active()
	if info == nil {
		return
	}
	player := info.Player
	t.cfg.AddBlacklist(player)
	_ = t.cfg.Save(t.cfgPath)

	if strings.ToLower(t.mgr.Forced()) == player {
		t.mgr.SetForced("")
	}

	t.mBlacklist.SetTitle(fmt.Sprintf("✓ Blacklisted: %s", player))
}

func openFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", path)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}
