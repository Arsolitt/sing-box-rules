package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRanges(t *testing.T) {
	response := IPInfoResponse{
		Domain:      "github.com",
		RedirectsTo: nil,
		NumRanges:   2,
		Ranges:      []string{"1.2.3.0/24", "2401:cf20::/32"},
	}
	body, _ := json.Marshal(response)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/widget/demo/github.com" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("dataset") != "ranges" {
			t.Errorf("missing dataset query param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer server.Close()

	result, err := FetchRanges(server.URL, "github.com")
	if err != nil {
		t.Fatal(err)
	}
	if result.Domain != "github.com" {
		t.Errorf("expected domain 'github.com', got %q", result.Domain)
	}
	if len(result.Ranges) != 2 {
		t.Errorf("expected 2 ranges, got %d", len(result.Ranges))
	}
}

func TestFetchRangesRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	_, err := FetchRanges(server.URL, "github.com")
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !IsRateLimitError(err) {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestFetchRangesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := FetchRanges(server.URL, "github.com")
	if err == nil {
		t.Fatal("expected error on 500")
	}
	if IsRateLimitError(err) {
		t.Error("500 should not be a rate limit error")
	}
}
