// Package wayzn provides a Go client for controlling the Wayzn Smart Pet Door.
//
// API endpoints are discovered by intercepting the Wayzn mobile app's traffic
// using the capture/wayzn_capture.py mitmproxy script. Once traffic is captured,
// run capture/analyze.py --generate-go to produce client_generated.go with the
// actual endpoint implementations.
package wayzn

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DoorState represents the current state of the Wayzn door.
type DoorState string

const (
	DoorOpen    DoorState = "open"
	DoorClosed  DoorState = "closed"
	DoorMoving  DoorState = "moving"
	DoorUnknown DoorState = "unknown"
)

// DoorStatus holds the status information returned by the Wayzn API.
type DoorStatus struct {
	State     DoorState `json:"state"`
	Online    bool      `json:"online"`
	Timestamp time.Time `json:"timestamp"`
	Raw       any       `json:"raw,omitempty"`
}

// Config holds the configuration for a Wayzn API client.
type Config struct {
	// BaseURL is the Wayzn API base URL (discovered via traffic capture).
	BaseURL string `json:"base_url"`

	// Token is the authentication token (discovered via traffic capture).
	Token string `json:"token"`

	// DeviceID is the door's device identifier (discovered via traffic capture).
	DeviceID string `json:"device_id"`

	// Timeout for HTTP requests.
	Timeout time.Duration `json:"timeout"`

	// AuthHeader is the header name used for authentication (e.g., "Authorization").
	AuthHeader string `json:"auth_header"`
}

// Client communicates with the Wayzn cloud API.
type Client struct {
	config Config
	http   *http.Client
}

// NewClient creates a new Wayzn API client.
func NewClient(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: cfg.Timeout},
	}
}

// doRequest executes an authenticated HTTP request against the Wayzn API.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.config.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("wayzn: create request: %w", err)
	}

	req.Header.Set(c.config.AuthHeader, c.config.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wayzn: request %s %s: %w", method, path, err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("wayzn: %s %s returned HTTP %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	return resp, nil
}

// decodeResponse reads and decodes a JSON response body.
func decodeResponse(resp *http.Response, v any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

// LoadConfigFromFile loads a Wayzn client config from a JSON file.
func LoadConfigFromFile(path string) (Config, error) {
	var cfg Config
	f, err := openFile(path)
	if err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// openFile is a testable file opener.
var openFile = func(path string) (io.ReadCloser, error) {
	return (&fileOpener{}).Open(path)
}

type fileOpener struct{}

func (fo *fileOpener) Open(path string) (io.ReadCloser, error) {
	return http.Dir(".").Open(path)
}
