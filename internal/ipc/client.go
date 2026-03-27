package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// Client is a thin HTTP client over Unix socket, used by plugin hook scripts.
type Client struct {
	socketPath string
	httpClient *http.Client
}

// NewClient creates an IPC client that connects to the daemon.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.DialTimeout("unix", socketPath, 2*time.Second)
				},
			},
			Timeout: 10 * time.Second,
		},
	}
}

// SendEvent sends an agent event to the daemon and returns the classification.
func (c *Client) SendEvent(ctx context.Context, evt agent.AgentEvent) (*agent.EventResponse, error) {
	var resp agent.EventResponse
	if err := c.post(ctx, "/event", evt, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PollSlap checks for a buffered slap event. Returns nil if no slap.
func (c *Client) PollSlap(ctx context.Context) (*agent.SlapEvent, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://pillow/slap", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	var evt agent.SlapEvent
	if err := json.NewDecoder(resp.Body).Decode(&evt); err != nil {
		return nil, err
	}
	return &evt, nil
}

// Narrate requests immediate narration of a string.
func (c *Client) Narrate(ctx context.Context, text string) error {
	return c.post(ctx, "/narrate", agent.NarrateRequest{Text: text}, nil)
}

// GetSummary requests the current rolling summary.
func (c *Client) GetSummary(ctx context.Context) (*agent.SummaryResponse, error) {
	var resp agent.SummaryResponse
	if err := c.get(ctx, "/summary", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SessionStart signals session start.
func (c *Client) SessionStart(ctx context.Context, req agent.SessionStartRequest) error {
	return c.post(ctx, "/session/start", req, nil)
}

// SessionEnd signals session end and returns cost + summary.
func (c *Client) SessionEnd(ctx context.Context, req agent.SessionEndRequest) (*agent.SessionEndResponse, error) {
	var resp agent.SessionEndResponse
	if err := c.post(ctx, "/session/end", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStatus returns daemon status info.
func (c *Client) GetStatus(ctx context.Context) (*agent.StatusResponse, error) {
	var resp agent.StatusResponse
	if err := c.get(ctx, "/status", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Ping checks if the daemon is running.
func (c *Client) Ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := c.GetStatus(ctx)
	return err == nil
}

func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "http://pillow"+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to pillow daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://pillow"+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connecting to pillow daemon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}
