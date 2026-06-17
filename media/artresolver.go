package media

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var artClient = &http.Client{Timeout: 2 * time.Second}

// itunesResult holds the fields we backfill from an iTunes lookup.
type itunesResult struct {
	artURL string
	album  string
}

// itunesCache memoises iTunes lookups keyed by the search term so we don't hit
// the network on every 2s poll for the same track.
var (
	itunesCache   = map[string]itunesResult{}
	itunesCacheMu sync.Mutex
)

// itunesArt resolves cover art and album name from artist/album/title metadata
// via Apple's keyless iTunes Search API. Used on platforms (Windows SMTC) where
// the media API gives no art URL and may not provide an album name either.
func itunesArt(artist, album, title string) itunesResult {
	term := strings.TrimSpace(artist + " " + album)
	if album == "" {
		term = strings.TrimSpace(artist + " " + title)
	}
	if term == "" {
		return itunesResult{}
	}

	itunesCacheMu.Lock()
	if cached, ok := itunesCache[term]; ok {
		itunesCacheMu.Unlock()
		return cached
	}
	itunesCacheMu.Unlock()

	entity := "album"
	if album == "" {
		entity = "song"
	}
	q := url.Values{}
	q.Set("term", term)
	q.Set("entity", entity)
	q.Set("limit", "1")

	var result itunesResult
	resp, err := artClient.Get("https://itunes.apple.com/search?" + q.Encode())
	if err == nil {
		defer resp.Body.Close()
		var payload struct {
			Results []struct {
				ArtworkURL100  string `json:"artworkUrl100"`
				CollectionName string `json:"collectionName"`
			} `json:"results"`
		}
		if json.NewDecoder(resp.Body).Decode(&payload) == nil && len(payload.Results) > 0 {
			// upgrade the 100px thumbnail to 600px
			result.artURL = strings.Replace(payload.Results[0].ArtworkURL100, "100x100bb", "600x600bb", 1)
			result.album = payload.Results[0].CollectionName
		}
	}

	itunesCacheMu.Lock()
	itunesCache[term] = result // cache misses too, to avoid repeat lookups
	itunesCacheMu.Unlock()
	return result
}

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
