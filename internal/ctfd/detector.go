package ctfd

import (
	"fmt"
	"io"
	"net/http"
)

const (
	EditionEnterprise = "enterprise"
	EditionOpenSource = "open_source"
	EditionUnknown    = "unknown"
)

type Detector struct {
	baseURL string
	apiKey  string
}

func NewDetector(baseURL, apiKey string) *Detector {
	return &Detector{baseURL: baseURL, apiKey: apiKey}
}

func (d *Detector) Detect() (string, error) {
	if d.baseURL == "" {
		return EditionUnknown, nil
	}

	req, err := http.NewRequest(http.MethodGet, d.baseURL+"/api/v1/webhooks", nil)
	if err != nil {
		return EditionUnknown, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+d.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return EditionUnknown, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return EditionEnterprise, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return EditionOpenSource, nil
	}

	body, _ := io.ReadAll(resp.Body)
	return EditionUnknown, fmt.Errorf("unexpected response %d: %s", resp.StatusCode, string(body))
}
