package tmdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// newTestClient creates a Client pointed at the given test server.
func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c := &Client{
		apiKey:  "test-key",
		base:    srv.URL,
		http:    srv.Client(),
		limiter: rate.NewLimiter(rate.Inf, 1), // no rate limit in tests
	}
	return c, srv
}

func TestGet_Success(t *testing.T) {
	want := map[string]string{"hello": "world"}

	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Errorf("api_key = %q, want test-key", r.URL.Query().Get("api_key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(want)
	}))

	var got map[string]string
	if err := c.get(context.Background(), "/test", nil, &got); err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if got["hello"] != "world" {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGet_MergesParams(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "batman" {
			t.Errorf("query = %q, want batman", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("api_key") != "test-key" {
			t.Errorf("api_key missing")
		}
		json.NewEncoder(w).Encode(map[string]string{})
	}))

	var dest map[string]string
	params := map[string][]string{"query": {"batman"}}
	if err := c.get(context.Background(), "/search", params, &dest); err != nil {
		t.Fatalf("get() error: %v", err)
	}
}

func TestGet_Non2xxError(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"status_message":"not found"}`))
	}))

	var dest map[string]any
	err := c.get(context.Background(), "/missing", nil, &dest)
	if err == nil {
		t.Fatal("get() should error on 404")
	}
}

func TestGet_InvalidJSON(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))

	var dest map[string]any
	err := c.get(context.Background(), "/bad", nil, &dest)
	if err == nil {
		t.Fatal("get() should error on invalid JSON")
	}
}

func TestGet_CancelledContext(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{})
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var dest map[string]any
	err := c.get(ctx, "/test", nil, &dest)
	if err == nil {
		t.Fatal("get() should error on cancelled context")
	}
}

func TestGet_ServerError(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))

	var dest map[string]any
	err := c.get(context.Background(), "/error", nil, &dest)
	if err == nil {
		t.Fatal("get() should error on 500")
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("my-api-key")
	if c.apiKey != "my-api-key" {
		t.Errorf("apiKey = %q, want my-api-key", c.apiKey)
	}
	if c.base != baseURL {
		t.Errorf("base = %q, want %q", c.base, baseURL)
	}
	if c.http.Timeout != defaultTimeout {
		t.Errorf("timeout = %v, want %v", c.http.Timeout, defaultTimeout)
	}
}

func TestImageURL(t *testing.T) {
	tests := []struct {
		size, path, want string
	}{
		{PosterMedium, "/abc.jpg", imageBaseURL + "/w342/abc.jpg"},
		{BackdropLarge, "/bg.jpg", imageBaseURL + "/w1280/bg.jpg"},
		{PosterSmall, "", ""},
	}
	for _, tt := range tests {
		got := ImageURL(tt.size, tt.path)
		if got != tt.want {
			t.Errorf("ImageURL(%q, %q) = %q, want %q", tt.size, tt.path, got, tt.want)
		}
	}
}

func TestGet_RateLimiter(t *testing.T) {
	calls := 0
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	// Override with a very slow limiter.
	c.limiter = rate.NewLimiter(rate.Every(1*time.Hour), 1)

	var dest map[string]string
	// First call uses the burst token.
	if err := c.get(context.Background(), "/test", nil, &dest); err != nil {
		t.Fatalf("first call should succeed: %v", err)
	}

	// Second call should block; cancel immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	err := c.get(ctx, "/test", nil, &dest)
	if err == nil {
		t.Fatal("second call should fail due to rate limit + timeout")
	}
}
