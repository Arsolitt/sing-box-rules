package internal

import (
	"encoding/json"
	"fmt"
	"os"
)

type DomainConfig struct {
	Name         string   `json:"name"`
	Domain       string   `json:"domain"`
	IntervalDays int      `json:"interval_days"`
	ExtraDomains []string `json:"extra_domains,omitempty"`
}

func LoadConfig(path string) ([]DomainConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var configs []DomainConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return configs, nil
}
