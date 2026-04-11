package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type IPInfoResponse struct {
	Domain      string   `json:"domain"`
	RedirectsTo *string  `json:"redirects_to"`
	NumRanges   int      `json:"num_ranges"`
	Ranges      []string `json:"ranges"`
}

type RateLimitError struct {
	StatusCode int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limited (HTTP %d)", e.StatusCode)
}

func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

func FetchRanges(baseURL, domain string) (*IPInfoResponse, error) {
	url := fmt.Sprintf("%s/widget/demo/%s?dataset=ranges", baseURL, domain)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request ipinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &RateLimitError{StatusCode: resp.StatusCode}
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ipinfo returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result IPInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode ipinfo response: %w", err)
	}

	return &result, nil
}
