// Package ctftime is a client for the CTFtime API.
package ctftime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	_ "image/gif"  // Decode GIF logos.
	_ "image/jpeg" // Decode JPEG logos.
	_ "image/png"  // Decode PNG logos.
)

// DefaultBaseURL is the root of the CTFtime API.
const DefaultBaseURL = "https://ctftime.org/api/v1"

// maxLogoSize limits how much of a logo is read.
const maxLogoSize = 5 << 20

// ErrNoResults is returned when a CTF has no results for the requested team.
var ErrNoResults = errors.New("no results")

var userAgents = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.5 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.53 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Windows; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/103.0.5060.114 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_10_5) AppleWebKit/603.3.8 (KHTML, like Gecko) Version/10.1.2 Safari/603.3.8",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
}

// Client calls the CTFtime API.
type Client struct {
	http    *http.Client
	baseURL string
}

// New returns a Client for the API at baseURL. Requests are bounded by a
// timeout: events are handled on the gateway goroutine, so a request that never
// returns would freeze the whole bot.
func New(baseURL string) *Client {
	return &Client{http: &http.Client{Timeout: 20 * time.Second}, baseURL: baseURL}
}

// get fetches url and returns the response if its status is 200.
func (c *Client) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", userAgents[rand.IntN(len(userAgents))]) //nolint:gosec // Not security sensitive.

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return resp, nil
}

func (c *Client) getJSON(ctx context.Context, url string, v any) error {
	resp, err := c.get(ctx, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// Logo downloads an image and returns its bytes and format, such as "png".
func (c *Client) Logo(ctx context.Context, url string) ([]byte, string, error) {
	resp, err := c.get(ctx, url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxLogoSize))
	if err != nil {
		return nil, "", fmt.Errorf("read logo: %w", err)
	}

	// DecodeConfig only reads the header, so a hostile image can't exhaust memory.
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode logo: %w", err)
	}
	return data, format, nil
}

// Event returns an event by its CTFtime ID.
func (c *Client) Event(ctx context.Context, id int) (Event, error) {
	var event Event
	err := c.getJSON(ctx, c.baseURL+"/events/"+strconv.Itoa(id)+"/", &event)
	return event, err
}

// Upcoming returns the next events, leaving out the onsite ones unless onsite is true.
func (c *Client) Upcoming(ctx context.Context, limit int, onsite bool) ([]Event, error) {
	var events []Event
	if err := c.getJSON(ctx, c.baseURL+"/events/?limit="+strconv.Itoa(limit), &events); err != nil {
		return nil, err
	}
	if !onsite {
		events = slices.DeleteFunc(events, func(e Event) bool { return e.Onsite })
	}
	return events, nil
}

// Result is the standing of a team in a CTF.
type Result struct {
	Place  int
	Score  int
	Solves int
}

// Results returns the standing of a team in a CTF held in year, or ErrNoResults.
func (c *Client) Results(ctx context.Context, ctftimeID int64, year int, teamID int64) (Result, error) {
	var events map[string]struct {
		Scores []struct {
			TeamID int64  `json:"team_id"`
			Place  int    `json:"place"`
			Points string `json:"points"`
			Solves int    `json:"solves"`
		} `json:"scores"`
	}
	if err := c.getJSON(ctx, fmt.Sprintf("%s/results/%d/", c.baseURL, year), &events); err != nil {
		return Result{}, err
	}

	for _, score := range events[strconv.FormatInt(ctftimeID, 10)].Scores {
		if score.TeamID != teamID {
			continue
		}
		points, err := strconv.ParseFloat(strings.TrimSpace(score.Points), 64)
		if err != nil {
			return Result{}, fmt.Errorf("parse score: %w", err)
		}
		return Result{Place: score.Place, Score: int(points), Solves: score.Solves}, nil
	}
	return Result{}, ErrNoResults
}
