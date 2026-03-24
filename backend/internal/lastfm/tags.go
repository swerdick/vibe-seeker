package lastfm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MinTagCount is the minimum Last.fm tag relevance score (0-100) to keep.
// Tags below this threshold are filtered as noise.
const MinTagCount = 20

// blocklist contains tags that are noise regardless of their count.
var blocklist = map[string]bool{
	"seen live": true,
}

// Tag represents a Last.fm artist tag with its relevance score.
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"` // 0-100 relevance score relative to the artist's top tag
}

type topTagsResponse struct {
	TopTags struct {
		Tag []Tag `json:"tag"`
	} `json:"toptags"`
}

// FetchArtistTags returns the top tags for an artist, filtered by minimum
// count and blocklist. If the initial query returns no results and the artist
// name contains diacritics, retries with stripped diacritics.
func (c *Client) FetchArtistTags(ctx context.Context, artistName string) ([]Tag, error) {
	tags, err := c.fetchTags(ctx, artistName)
	if err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		return tags, nil
	}

	// Retry with stripped diacritics if the name contains non-ASCII.
	stripped := stripDiacritics(artistName)
	if stripped != artistName {
		slog.Info("retrying lastfm with stripped diacritics", "original", artistName, "stripped", stripped)
		tags, err = c.fetchTags(ctx, stripped)
		if err != nil {
			return nil, err
		}
		return tags, nil
	}

	return nil, nil
}

func (c *Client) fetchTags(ctx context.Context, artistName string) ([]Tag, error) {
	params := url.Values{
		"method":      {"artist.gettoptags"},
		"artist":      {artistName},
		"api_key":     {c.APIKey},
		"format":      {"json"},
		"autocorrect": {"1"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lastfm artist.gettoptags returned %d", resp.StatusCode)
	}

	var result topTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	filtered := filterTags(result.TopTags.Tag)
	slog.Info("lastfm tags fetched", "artist", artistName, "raw", len(result.TopTags.Tag), "filtered", len(filtered))
	return filtered, nil
}

// stripDiacritics removes accent marks and diacritical marks from a string.
// e.g., "rüfüs" → "rufus", "Beyoncé" → "Beyonce"
func stripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) { // Mn = Mark, Nonspacing (combining diacriticals)
			b.WriteRune(r)
		}
	}
	return b.String()
}

func filterTags(tags []Tag) []Tag {
	var filtered []Tag
	for _, t := range tags {
		if t.Count < MinTagCount {
			continue
		}
		if blocklist[strings.ToLower(t.Name)] {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}
