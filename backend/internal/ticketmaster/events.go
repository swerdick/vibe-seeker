package ticketmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Event represents a Ticketmaster event with the fields we need.
type Event struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	URL             string           `json:"url"`
	Dates           EventDates       `json:"dates"`
	PriceRanges     []PriceRange     `json:"priceRanges"`
	Classifications []Classification `json:"classifications"`
	Images          []Image          `json:"images"`
	Embedded        EventEmbedded    `json:"_embedded"`
}

type EventDates struct {
	Start  EventStart  `json:"start"`
	Status EventStatus `json:"status"`
}

type EventStart struct {
	DateTime  string `json:"dateTime"`
	LocalDate string `json:"localDate"`
	LocalTime string `json:"localTime"`
}

type EventStatus struct {
	Code string `json:"code"`
}

type PriceRange struct {
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
	Currency string  `json:"currency"`
}

// Classification holds Ticketmaster's hierarchical genre taxonomy.
type Classification struct {
	Segment  NamedEntity `json:"segment"`
	Genre    NamedEntity `json:"genre"`
	SubGenre NamedEntity `json:"subGenre"`
}

type EventEmbedded struct {
	Venues      []Venue      `json:"venues"`
	Attractions []Attraction `json:"attractions"`
}

// Attraction represents a performer/artist on an event.
type Attraction struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Classifications []Classification `json:"classifications"`
	Images          []Image          `json:"images"`
}

type eventSearchResponse struct {
	Embedded struct {
		Events []Event `json:"events"`
	} `json:"_embedded"`
	Page Page `json:"page"`
}

// EventSearchOptions holds parameters for searching events.
type EventSearchOptions struct {
	VenueID            string
	ClassificationName string
	Sort               string
	Size               int
	StartDateTime      string // RFC3339 format
}

// SearchEvents fetches events matching the given options, auto-paginating up to
// the Ticketmaster hard limit of 1,000 results.
func (c *Client) SearchEvents(ctx context.Context, opts EventSearchOptions) ([]Event, error) {
	if opts.Size == 0 {
		opts.Size = 200
	}
	if opts.Sort == "" {
		opts.Sort = "date,asc"
	}
	if opts.ClassificationName == "" {
		opts.ClassificationName = "music"
	}

	var all []Event
	for page := 0; ; page++ {
		if opts.Size*(page) >= 1000 {
			break
		}

		params := url.Values{
			"apikey":             {c.APIKey},
			"classificationName": {opts.ClassificationName},
			"size":               {fmt.Sprintf("%d", opts.Size)},
			"page":               {fmt.Sprintf("%d", page)},
			"sort":               {opts.Sort},
		}
		if opts.VenueID != "" {
			params.Set("venueId", opts.VenueID)
		}
		if opts.StartDateTime != "" {
			params.Set("startDateTime", opts.StartDateTime)
		}

		reqURL := fmt.Sprintf("%s/events.json?%s", c.BaseURL, params.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("sending request: %w", err)
		}

		var result eventSearchResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, ErrRateLimited
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("ticketmaster events endpoint returned %d", resp.StatusCode)
		}

		all = append(all, result.Embedded.Events...)

		if page >= result.Page.TotalPages-1 {
			break
		}
	}

	return all, nil
}
