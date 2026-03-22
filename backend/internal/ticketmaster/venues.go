package ticketmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// RawJSON holds arbitrary JSON for fields with variable structure.
type RawJSON = json.RawMessage

// Venue represents a Ticketmaster venue with the fields we need.
type Venue struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	URL           string        `json:"url"`
	Location      VenueLocation `json:"location"`
	Address       VenueAddress  `json:"address"`
	City          VenueCity     `json:"city"`
	State         VenueState    `json:"state"`
	Images        []Image       `json:"images"`
	BoxOfficeInfo RawJSON       `json:"boxOfficeInfo"`
	ParkingDetail string        `json:"parkingDetail"`
	GeneralInfo   RawJSON       `json:"generalInfo"`
	Ada           RawJSON       `json:"ada"`
	Country       VenueCountry  `json:"country"`
}

type VenueLocation struct {
	Latitude  string `json:"latitude"`
	Longitude string `json:"longitude"`
}

type VenueAddress struct {
	Line1 string `json:"line1"`
}

type VenueCountry struct {
	CountryCode string `json:"countryCode"`
}

type VenueCity struct {
	Name string `json:"name"`
}

type VenueState struct {
	StateCode string `json:"stateCode"`
}

type venueSearchResponse struct {
	Embedded struct {
		Venues []Venue `json:"venues"`
	} `json:"_embedded"`
	Page Page `json:"page"`
}

// VenueSearchOptions holds parameters for searching venues.
type VenueSearchOptions struct {
	GeoPoint string // "lat,lng" e.g., "40.7128,-74.0060"
	Radius   string // e.g., "15" (miles)
	Unit     string // "miles" or "km", defaults to "miles"
	Size     int
}

// SearchVenues fetches all venues for the given DMA, auto-paginating up to
// the Ticketmaster hard limit of 1,000 results.
func (c *Client) SearchVenues(ctx context.Context, opts VenueSearchOptions) ([]Venue, error) {
	if opts.Size == 0 {
		opts.Size = 200
	}

	var all []Venue
	for page := 0; ; page++ {
		if opts.Size*(page) >= 1000 {
			break
		}

		unit := opts.Unit
		if unit == "" {
			unit = "miles"
		}
		params := url.Values{
			"apikey": {c.APIKey},
			"size":   {fmt.Sprintf("%d", opts.Size)},
			"page":   {fmt.Sprintf("%d", page)},
			"locale": {"*"},
		}
		if opts.GeoPoint != "" {
			params.Set("geoPoint", opts.GeoPoint)
			params.Set("radius", opts.Radius)
			params.Set("unit", unit)
		}
		reqURL := fmt.Sprintf("%s/venues.json?%s", c.BaseURL, params.Encode())

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending request: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, ErrRateLimited
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ticketmaster venues endpoint returned %d", resp.StatusCode)
		}

		var result venueSearchResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		all = append(all, result.Embedded.Venues...)

		if page >= result.Page.TotalPages-1 {
			break
		}
	}

	return all, nil
}

// Lat returns the venue's latitude as a float64. Returns 0 if parsing fails.
func (v *Venue) Lat() float64 {
	f, _ := strconv.ParseFloat(v.Location.Latitude, 64)
	return f
}

// Lng returns the venue's longitude as a float64. Returns 0 if parsing fails.
func (v *Venue) Lng() float64 {
	f, _ := strconv.ParseFloat(v.Location.Longitude, 64)
	return f
}
