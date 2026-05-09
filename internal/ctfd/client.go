package ctfd

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

type Client struct {
	baseURL string
	apiKey  string
	log     *slog.Logger
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		log:     slog.With("component", "ctfd"),
	}
}

type userMeResponse struct {
	Data struct {
		ID int `json:"id"`
	} `json:"data"`
}

func (c *Client) VerifyToken(token string) (int, error) {
	if c == nil || c.baseURL == "" {
		return 0, fmt.Errorf("ctfd not configured")
	}

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/api/v1/users/me", nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}

	var ures userMeResponse
	if err := json.Unmarshal(body, &ures); err != nil {
		return 0, fmt.Errorf("unmarshal: %w", err)
	}

	if ures.Data.ID == 0 {
		return 0, fmt.Errorf("invalid token")
	}

	return ures.Data.ID, nil
}
