package ticketmaster

import (
	"errors"
	"net/http"
)

// ErrRateLimited is returned when the Ticketmaster API responds with 429.
var ErrRateLimited = errors.New("ticketmaster: rate limited (429)")

const DefaultBaseURL = "https://app.ticketmaster.com/discovery/v2"

// Client handles communication with the Ticketmaster Discovery API v2.
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

// NamedEntity is a common Ticketmaster pattern for objects with an ID and name.
type NamedEntity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Image represents a Ticketmaster image.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Page holds Ticketmaster pagination metadata.
type Page struct {
	Size          int `json:"size"`
	TotalElements int `json:"totalElements"`
	TotalPages    int `json:"totalPages"`
	Number        int `json:"number"`
}
