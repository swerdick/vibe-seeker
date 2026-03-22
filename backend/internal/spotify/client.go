package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	DefaultAuthURL       = "https://accounts.spotify.com/authorize"
	DefaultTokenURL      = "https://accounts.spotify.com/api/token"
	DefaultMeURL         = "https://api.spotify.com/v1/me"
	DefaultTopArtistsURL = "https://api.spotify.com/v1/me/top/artists"
	Scopes               = "user-top-read"
)

// Client handles communication with the Spotify API.
type Client struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	AuthURL      string
	TokenURL     string
	MeURL        string
	TopArtistsURL string
	HTTPClient   *http.Client
}

// Profile holds the subset of Spotify user data the app needs.
type Profile struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// TokenResponse holds the tokens returned by the Spotify token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func NewClient(clientID, clientSecret, redirectURI string) *Client {
	return &Client{
		ClientID:      clientID,
		ClientSecret:  clientSecret,
		RedirectURI:   redirectURI,
		AuthURL:       DefaultAuthURL,
		TokenURL:      DefaultTokenURL,
		MeURL:         DefaultMeURL,
		TopArtistsURL: DefaultTopArtistsURL,
		HTTPClient:    http.DefaultClient,
	}
}

// AuthorizeURL builds the Spotify OAuth authorization URL for the given state parameter.
func (c *Client) AuthorizeURL(state string) string {
	params := url.Values{
		"client_id":     {c.ClientID},
		"response_type": {"code"},
		"redirect_uri":  {c.RedirectURI},
		"scope":         {Scopes},
		"state":         {state},
	}
	return c.AuthURL + "?" + params.Encode()
}

// ExchangeCode trades an authorization code for Spotify tokens.
func (c *Client) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {c.RedirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.ClientID, c.ClientSecret)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("spotify token endpoint returned %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &tokenResp, nil
}

// FetchProfile retrieves the authenticated user's Spotify profile.
func (c *Client) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.MeURL, nil)
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
