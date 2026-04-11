package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "domains.json")

	config := []DomainConfig{
		{
			Name:         "github",
			Domain:       "github.com",
			IntervalDays: 30,
			ExtraDomains: []string{"github.io", "ghcr.io"},
		},
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(loaded))
	}
	if loaded[0].Name != "github" {
		t.Errorf("expected name 'github', got %q", loaded[0].Name)
	}
	if loaded[0].Domain != "github.com" {
		t.Errorf("expected domain 'github.com', got %q", loaded[0].Domain)
	}
	if loaded[0].IntervalDays != 30 {
		t.Errorf("expected interval 30, got %d", loaded[0].IntervalDays)
	}
	if len(loaded[0].ExtraDomains) != 2 {
		t.Errorf("expected 2 extra domains, got %d", len(loaded[0].ExtraDomains))
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/domains.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "domains.json")
	if err := os.WriteFile(configPath, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
