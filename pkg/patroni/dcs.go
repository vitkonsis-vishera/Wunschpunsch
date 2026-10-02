package patroni

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// doRequest выполняет HTTP-запрос, перебирая доступные эндпоинты Patroni
func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	if len(c.endpoints) == 0 {
		return nil, fmt.Errorf("no patroni endpoints configured")
	}

	var lastErr error
	for _, endpoint := range c.endpoints {
		url := strings.TrimRight(endpoint, "/") + path
		var bodyReader io.Reader
		if body != nil {
			bodyReader = bytes.NewReader(body)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
		if err != nil {
			lastErr = err
			continue
		}

		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed to reach any patroni endpoint: %w", lastErr)
}

// GetDCSConfig получает текущую динамическую конфигурацию Patroni в виде formatted JSON
func (c *Client) GetDCSConfig(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/config", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

// UpdateDCSConfig обновляет DCS-конфигурацию Patroni (PATCH /config)
func (c *Client) UpdateDCSConfig(ctx context.Context, patchJSON string) error {
	resp, err := c.doRequest(ctx, http.MethodPatch, "/config", []byte(patchJSON))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to patch config (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// RestartNode отправляет запрос на перезапуск конкретного узла
func (c *Client) RestartNode(ctx context.Context, nodeName string) error {
	jsonPayload := []byte(fmt.Sprintf(`{"node":"%s"}`, nodeName))
	resp, err := c.doRequest(ctx, http.MethodPost, "/restart", jsonPayload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("restart failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// ReloadNode отправляет запрос на перезагрузку конфигурации конкретного узла
func (c *Client) ReloadNode(ctx context.Context, nodeName string) error {
	jsonPayload := []byte(fmt.Sprintf(`{"node":"%s"}`, nodeName))
	resp, err := c.doRequest(ctx, http.MethodPost, "/reload", jsonPayload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reload failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}
