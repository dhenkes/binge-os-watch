package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

const (
	baseURL        = "https://api.themoviedb.org/3"
	imageBaseURL   = "https://image.tmdb.org/t/p"
	defaultTimeout = 10 * time.Second
)

// Image size constants for building TMDB image URLs.
const (
	PosterSmall    = "w92"
	PosterMedium   = "w342"
	PosterLarge    = "w780"
	BackdropMedium = "w780"
	BackdropLarge  = "w1280"
)

// Client wraps the TMDB v3 API with rate limiting.
type Client struct {
	apiKey  string
	base    string // API base URL (overridable for tests)
	http    *http.Client
	limiter *rate.Limiter
}

// NewClient creates a TMDB client with the given API key.
// Rate limited to ~40 requests per 10 seconds (TMDB's limit).
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		base:   baseURL,
		http: &http.Client{
			Timeout: defaultTimeout,
		},
		limiter: rate.NewLimiter(rate.Every(250*time.Millisecond), 4), // 4 req/s burst
	}
}

// ImageURL builds a full TMDB image CDN URL from a path and size.
// Returns empty string if path is empty.
func ImageURL(size, path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s%s", imageBaseURL, size, path)
}

// get performs a rate-limited GET request to the TMDB API and decodes the
// JSON response into dest.
func (c *Client) get(ctx context.Context, path string, params url.Values, dest any) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("tmdb: rate limiter: %w", err)
	}

	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.apiKey)

	fullURL := c.base + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return fmt.Errorf("tmdb: creating request: %w", err)
	}

	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = c.http.Do(req)
		if err != nil {
			return fmt.Errorf("tmdb: executing request: %w", err)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := 2 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					retryAfter = time.Duration(secs) * time.Second
				}
			}
			resp.Body.Close()
			slog.Warn("tmdb: rate limited, backing off", "retry_after", retryAfter, "attempt", attempt+1)
			select {
			case <-time.After(retryAfter):
				// Rebuild request since body was consumed.
				req, _ = http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		break
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("tmdb: %s %s: status %d: %s", http.MethodGet, path, resp.StatusCode, body)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("tmdb: decoding response: %w", err)
	}

	return nil
}
