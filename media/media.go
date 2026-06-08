package media

import "time"

type Info struct {
	Player    string
	Title     string
	Artist    string
	Album     string
	ArtURL    string
	Playing   bool
	StartedAt time.Time
	Duration  time.Duration // 0 = unknown
}

// Source polls a platform media API and returns all currently active players.
type Source interface {
	Players() ([]Info, error)
}
