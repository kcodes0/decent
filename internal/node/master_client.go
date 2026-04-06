package node

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/kcodes0/decent/internal/protocol"
)

type masterClient struct {
	baseURL string
	client  *http.Client
}

func newMasterClient(baseURL string) *masterClient {
	return &masterClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *masterClient) Register(ctx context.Context, node protocol.RegisteredNode) error {
	var resp protocol.RegisterResponse
	return c.postJSON(ctx, "/api/register", protocol.RegisterRequest{Node: node}, &resp)
}

func (c *masterClient) Heartbeat(ctx context.Context, req protocol.HeartbeatRequest) error {
	var resp map[string]any
	return c.postJSON(ctx, "/api/heartbeat", req, &resp)
}

func (c *masterClient) postJSON(ctx context.Context, path string, payload any, out any) error {
	if c == nil || c.baseURL == "" {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		if len(msg) > 0 {
			return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(msg)))
		}
		return fmt.Errorf("%s", resp.Status)
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return err
	}
	return nil
}
