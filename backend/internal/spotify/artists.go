package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Artist represents a Spotify artist from the top artists endpoint.
type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TopArtistsResponse holds the response from the top artists endpoint.
type TopArtistsResponse struct {
	Items []Artist `json:"items"`
}

// FetchTopArtists retrieves the user's top artists for the given time range.
// timeRange must be one of: short_term, medium_term, long_term.
func (c *Client) FetchTopArtists(ctx context.Context, accessToken, timeRange string, limit int) (*TopArtistsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.TopArtistsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Set("time_range", timeRange)
	q.Set("limit", strconv.Itoa(limit))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify top artists endpoint returned %d", resp.StatusCode)
	}

	var result TopArtistsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}
