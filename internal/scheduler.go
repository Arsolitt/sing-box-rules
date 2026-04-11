package internal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func DetermineOutdated(client *GitClient, configs []DomainConfig) ([]DomainConfig, error) {
	now := time.Now()

	var outdated []DomainConfig
	for _, cfg := range configs {
		lastCommitDate, err := getLastCommitDate(client, cfg.Name+".srs")
		if err != nil || lastCommitDate.IsZero() {
			outdated = append(outdated, cfg)
			continue
		}

		threshold := lastCommitDate.AddDate(0, 0, cfg.IntervalDays)
		if now.After(threshold) {
			outdated = append(outdated, cfg)
		}
	}

	sort.Slice(outdated, func(i, j int) bool {
		return outdated[i].IntervalDays < outdated[j].IntervalDays
	})

	return outdated, nil
}

func getLastCommitDate(client *GitClient, filename string) (time.Time, error) {
	out, err := client.RunGit("log", "--format=%at", "--follow", "--", filename)
	if err != nil {
		return time.Time{}, nil
	}

	out = strings.TrimSpace(out)
	if out == "" {
		return time.Time{}, nil
	}

	seconds, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse commit date: %w", err)
	}

	return time.Unix(seconds, 0), nil
}
