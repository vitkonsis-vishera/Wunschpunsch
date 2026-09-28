package patroni

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// --- Patroni API Data Structures ---

type Member struct {
	Name           string `json:"name"`
	Role           string `json:"role"`  // leader, sync_standby, replica, demoted
	State          string `json:"state"` // running, streaming, stopped
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Timeline       int    `json:"timeline"`
	Lag            uint64 `json:"lag,omitempty"`
	PendingRestart bool   `json:"pending_restart,omitempty"`
}

type ClusterStatus struct {
	Scope               string   `json:"scope"`
	Members             []Member `json:"members"`
	ScheduledSwitchover *struct {
		At   string `json:"at"`
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"scheduled_switchover,omitempty"`
	Pause bool `json:"pause"`
}

type SwitchoverPayload struct {
	Leader    string `json:"leader,omitempty"`
	Candidate string `json:"candidate,omitempty"`
	Scheduled string `json:"scheduled,omitempty"`
}

type HistoryEntry struct {
	TL        int    `json:"timeline"`
	LSN       uint64 `json:"lsn"`
	Reason    string `json:"reason"`
	Timestamp string `json:"timestamp"`
}

// --- Patroni Client ---

type Client struct {
	httpClient *http.Client
	endpoints  []string
	mu         sync.RWMutex
}

func NewClient(initialEndpoints []string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		endpoints: initialEndpoints,
	}
}

func (c *Client) GetClusterState(ctx context.Context) (*ClusterStatus, error) {
	c.mu.RLock()
	endpoints := make([]string, len(c.endpoints))
	copy(endpoints, c.endpoints)
	c.mu.RUnlock()

	var lastErr error
	for _, ep := range endpoints {
		url := fmt.Sprintf("%s/cluster", ep)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("node %s returned status %d", ep, resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var status ClusterStatus
		if err := json.Unmarshal(body, &status); err != nil {
			lastErr = err
			continue
		}

		c.updateEndpointsFromStatus(&status)
		return &status, nil
	}

	return nil, fmt.Errorf("failed to fetch cluster status: %w", lastErr)
}

func (c *Client) GetHistory(ctx context.Context) ([]HistoryEntry, error) {
	c.mu.RLock()
	endpoints := make([]string, len(c.endpoints))
	copy(endpoints, c.endpoints)
	c.mu.RUnlock()

	var lastErr error
	for _, ep := range endpoints {
		url := fmt.Sprintf("%s/history", ep)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("node %s returned status %d", ep, resp.StatusCode)
			continue
		}

		var history []HistoryEntry
		err = json.NewDecoder(resp.Body).Decode(&history)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		return history, nil
	}

	return nil, fmt.Errorf("failed to fetch patroni history: %w", lastErr)
}

func (c *Client) updateEndpointsFromStatus(status *ClusterStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newEndpoints := make([]string, 0, len(status.Members))
	for _, m := range status.Members {
		ep := fmt.Sprintf("http://%s:8008", m.Host)
		newEndpoints = append(newEndpoints, ep)
	}

	if len(newEndpoints) > 0 {
		c.endpoints = newEndpoints
	}
}

func (c *Client) postAPIToHost(ctx context.Context, hostEndpoint string, path string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ep := hostEndpoint
	if !strings.HasPrefix(ep, "http://") && !strings.HasPrefix(ep, "https://") {
		ep = "http://" + ep
	}
	if !strings.Contains(strings.TrimPrefix(strings.TrimPrefix(ep, "http://"), "https://"), ":") {
		ep = ep + ":8008"
	}

	url := fmt.Sprintf("%s%s", ep, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("node %s returned status %d: %s", ep, resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) postAPI(ctx context.Context, path string, payload interface{}, preferredHost string) error {
	if preferredHost != "" {
		err := c.postAPIToHost(ctx, preferredHost, path, payload)
		if err == nil {
			return nil
		}
	}

	c.mu.RLock()
	endpoints := make([]string, len(c.endpoints))
	copy(endpoints, c.endpoints)
	c.mu.RUnlock()

	var lastErr error
	for _, ep := range endpoints {
		err := c.postAPIToHost(ctx, ep, path, payload)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	if lastErr != nil {
		return fmt.Errorf("failed API POST to %s: %w", path, lastErr)
	}
	return fmt.Errorf("failed API POST to %s: no endpoints available", path)
}

func (c *Client) Switchover(ctx context.Context, leaderNode, candidateNode string, leaderHost string) error {
	payload := SwitchoverPayload{
		Leader:    leaderNode,
		Candidate: candidateNode,
	}
	return c.postAPI(ctx, "/switchover", payload, leaderHost)
}

func (c *Client) Failover(ctx context.Context, candidateNode string, leaderHost string) error {
	payload := map[string]string{}
	if candidateNode != "" {
		payload["candidate"] = candidateNode
	}
	return c.postAPI(ctx, "/failover", payload, leaderHost)
}

func (c *Client) Reinitialize(ctx context.Context, targetNode string, targetHost string) error {
	payload := map[string]interface{}{
		"force": true,
	}
	return c.postAPI(ctx, "/reinitialize", payload, targetHost)
}

func (c *Client) TogglePause(ctx context.Context, leaderHost string) (bool, error) {
	status, err := c.GetClusterState(ctx)
	if err != nil {
		return false, err
	}

	endpoint := "/pause"
	if status.Pause {
		endpoint = "/resume"
	}

	err = c.postAPI(ctx, endpoint, map[string]string{}, leaderHost)
	if err != nil {
		return status.Pause, err
	}

	return !status.Pause, nil
}
