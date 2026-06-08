package media

import (
	"strings"
	"sync"

	"media-rpc/config"
)

type Manager struct {
	mu      sync.RWMutex
	cfg     *config.Config
	sources []Source
	forced  string
	last    []Info
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:     cfg,
		sources: platformSources(),
	}
}

func (m *Manager) SetForced(id string) {
	m.mu.Lock()
	m.forced = strings.ToLower(id)
	m.mu.Unlock()
}

func (m *Manager) Forced() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.forced
}

// Poll queries all sources and stores results. Called from the background loop.
func (m *Manager) Poll() {
	var all []Info
	for _, src := range m.sources {
		infos, err := src.Players()
		if err != nil {
			continue
		}
		all = append(all, infos...)
	}
	m.mu.Lock()
	m.last = all
	m.mu.Unlock()
}

// Players returns all players seen in the last poll (for display/cycling).
func (m *Manager) Players() []Info {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.last
}

// Active returns the best playing player after applying forced/priority/blacklist.
func (m *Manager) Active() *Info {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var candidates []Info
	for _, p := range m.last {
		if p.Playing && !m.cfg.IsBlacklisted(p.Player) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	if m.forced != "" {
		for i := range candidates {
			if candidates[i].Player == m.forced {
				return &candidates[i]
			}
		}
	}

	best := 0
	bestPrio := m.cfg.PriorityOf(candidates[0].Player)
	for i := 1; i < len(candidates); i++ {
		p := m.cfg.PriorityOf(candidates[i].Player)
		if p < bestPrio {
			bestPrio = p
			best = i
		}
	}
	return &candidates[best]
}
