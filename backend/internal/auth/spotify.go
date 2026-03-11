package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	DefaultAuthURL  = "https://accounts.spotify.com/authorize"
	DefaultTokenURL = "https://accounts.spotify.com/api/token"
	DefaultMeURL    = "https://api.spotify.com/v1/me"
	Scopes          = "user-top-read"
)

// SpotifyClient handles communication with the Spotify API for OAuth and profile retrieval.
type SpotifyClient struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthURL      string
	TokenURL     string
	MeURL        string
	HTTPClient   *http.Client
}

// Profile holds the subset of Spotify user data the app needs.
type Profile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func NewSpotifyClient(clientID, clientSecret, redirectURI string) *SpotifyClient {
	return &SpotifyClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURI:  redirectURI,
		AuthURL:      DefaultAuthURL,
		TokenURL:     DefaultTokenURL,
		MeURL:        DefaultMeURL,
		HTTPClient:   http.DefaultClient,
	}
}

// AuthorizeURL builds the Spotify OAuth authorization URL for the given state parameter.
func (c *SpotifyClient) AuthorizeURL(state string) string {
	params := url.Values{
		"client_id":     {c.ClientID},
		"response_type": {"code"},
		"redirect_uri":  {c.RedirectURI},
		"scope":         {Scopes},
		"state":         {state},
	}
	return c.AuthURL + "?" + params.Encode()
}

// ExchangeCode trades an authorization code for a Spotify access token.
func (c *SpotifyClient) ExchangeCode(code string) (string, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {c.RedirectURI},
	}

	req, err := http.NewRequest(http.MethodPost, c.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.ClientID, c.ClientSecret)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("spotify token endpoint returned %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decoding response: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// FetchProfile retrieves the authenticated user's Spotify profile.
func (c *SpotifyClient) FetchProfile(accessToken string) (*Profile, error) {
	req, err := http.NewRequest(http.MethodGet, c.MeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify me endpoint returned %d", resp.StatusCode)
	}

	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &profile, nil
}
