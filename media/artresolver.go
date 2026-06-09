package media

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

var artClient = &http.Client{Timeout: 2 * time.Second}

// resolveArtURL turns whatever MPRIS gives us into a publicly accessible
// HTTPS URL that Discord can load, or "" if that's not possible.
func resolveArtURL(rawURL, trackURL string) string {
	switch {
	case strings.HasPrefix(rawURL, "https://"), strings.HasPrefix(rawURL, "http://"):
		return upgradeResolution(rawURL)

	case strings.HasPrefix(rawURL, "file://"):
		// Local temp file — try service-specific APIs to get a real URL.
		return resolveFileArt(trackURL)
	}
	return ""
}

// upgradeResolution replaces Spotify's 300px image hash with the 640px one.
// The two hashes are fixed prefixes in Spotify CDN URLs; everything after is
// the track-specific image ID and stays unchanged.
func upgradeResolution(url string) string {
	return strings.ReplaceAll(url, "ab67616d00001e02", "ab67616d0000b273")
}

// resolveFileArt is called when MPRIS only has a local file:// path.
// Try tidal-hifi first (works for both the desktop app and its MPRIS bridge),
// then fall back to any other service we recognise from the track URL.
func resolveFileArt(trackURL string) string {
	if url := tidalHifiArtURL(); url != "" {
		return url
	}
	// future: add YouTube Music, Deezer, etc. resolvers keyed on trackURL
	return ""
}

// tidalHifiArtURL queries the tidal-hifi local API (port 47836).
// Returns the CDN image URL from the `image` field, or "" if unavailable.
func tidalHifiArtURL() string {
	resp, err := artClient.Get("http://localhost:47836/current")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var payload struct {
		Image string `json:"image"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	if strings.HasPrefix(payload.Image, "http") {
		return payload.Image
	}
	return ""
}
