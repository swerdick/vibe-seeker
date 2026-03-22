package lastfm

import "net/http"

const DefaultBaseURL = "https://ws.audioscrobbler.com/2.0/"

// Client handles communication with the Last.fm API.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey:     apiKey,
		BaseURL:    DefaultBaseURL,
		HTTPClient: http.DefaultClient,
	}
}
