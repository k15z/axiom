package wayzn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

// These endpoint implementations are placeholders. Once you capture actual
// Wayzn API traffic using the mitmproxy script, run:
//
//   python capture/analyze.py --generate-go
//
// This will generate client_generated.go with real endpoint paths and payloads
// based on your captured traffic. You can then replace these stubs.

// Open commands the Wayzn door to open.
func (c *Client) Open(ctx context.Context) error {
	// Common IoT patterns: the API likely uses one of:
	//   POST /api/v1/devices/{id}/commands  {"command": "open"}
	//   PUT  /api/v1/devices/{id}/state     {"state": "open"}
	//   POST /api/v1/doors/{id}/open
	//
	// The actual endpoint will be discovered via traffic capture.

	if c.config.DeviceID == "" {
		return fmt.Errorf("open: device_id not configured")
	}

	// Try the most common IoT command pattern
	path := fmt.Sprintf("/api/v1/devices/%s/commands", c.config.DeviceID)
	payload := map[string]string{"command": "open"}
	body, _ := json.Marshal(payload)

	resp, err := c.doRequest(ctx, "POST", path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	resp.Body.Close()
	return nil
}

// Close commands the Wayzn door to close.
func (c *Client) Close(ctx context.Context) error {
	if c.config.DeviceID == "" {
		return fmt.Errorf("close: device_id not configured")
	}

	path := fmt.Sprintf("/api/v1/devices/%s/commands", c.config.DeviceID)
	payload := map[string]string{"command": "close"}
	body, _ := json.Marshal(payload)

	resp, err := c.doRequest(ctx, "POST", path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}
	resp.Body.Close()
	return nil
}

// GetStatus retrieves the current state of the door.
func (c *Client) GetStatus(ctx context.Context) (*DoorStatus, error) {
	if c.config.DeviceID == "" {
		return nil, fmt.Errorf("status: device_id not configured")
	}

	path := fmt.Sprintf("/api/v1/devices/%s/status", c.config.DeviceID)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("status: %w", err)
	}

	var status DoorStatus
	if err := decodeResponse(resp, &status); err != nil {
		return nil, fmt.Errorf("status: decode: %w", err)
	}
	return &status, nil
}
