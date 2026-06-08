package config

import (
	"encoding/json"
	"os"
	"strings"
)

type Config struct {
	AppID       string            `json:"app_id"`
	Priority    []string          `json:"priority"`
	Blacklist   []string          `json:"blacklist"`
	// PlayerIcons maps player names to a Discord asset key or any HTTPS image URL.
	// Used as the small overlay icon on the album art. Example:
	//   "spotify": "spotify_logo"  (key uploaded to your Discord app's Rich Presence assets)
	//   "tidal":   "https://example.com/tidal.png"
	PlayerIcons map[string]string `json:"player_icons"`
	// SmallIcon is the fallback small icon when no player-specific one is set.
	// Defaults to a music note emoji image.
	SmallIcon   string            `json:"small_icon"`
}

func Default() *Config {
	return &Config{
		AppID:       "REPLACE_WITH_YOUR_DISCORD_APP_ID",
		Priority:    []string{},
		Blacklist:   []string{},
		PlayerIcons: map[string]string{},
		SmallIcon:   "https://cdn.jsdelivr.net/gh/twitter/twemoji@14.0.2/assets/72x72/1f3b5.png",
	}
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func (c *Config) IsBlacklisted(id string) bool {
	id = strings.ToLower(id)
	for _, b := range c.Blacklist {
		if strings.ToLower(b) == id {
			return true
		}
	}
	return false
}

func (c *Config) AddBlacklist(id string) {
	if !c.IsBlacklisted(id) {
		c.Blacklist = append(c.Blacklist, strings.ToLower(id))
	}
}

func (c *Config) PriorityOf(id string) int {
	id = strings.ToLower(id)
	for i, p := range c.Priority {
		if strings.ToLower(p) == id {
			return i
		}
	}
	return len(c.Priority)
}
