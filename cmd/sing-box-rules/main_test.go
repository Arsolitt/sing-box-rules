package main

import (
	"testing"

	"github.com/arsolitt/sing-box-rules/internal"
)

func TestBuildCommitMessage(t *testing.T) {
	mirror := func(files ...string) internal.MirrorResult {
		return internal.MirrorResult{Source: "src", Files: files}
	}
	custom := func(n int) []internal.CustomResult {
		out := make([]internal.CustomResult, n)
		return out
	}

	cases := []struct {
		name     string
		domains  []string
		custom   []internal.CustomResult
		external []internal.MirrorResult
		want     string
	}{
		{
			name: "nothing",
			want: "update: (no changes)",
		},
		{
			name:    "domains only",
			domains: []string{"amazon", "github"},
			want:    "update: amazon, github (2 domains)",
		},
		{
			name:   "custom only",
			custom: custom(3),
			want:   "update: custom (3 rules)",
		},
		{
			name:     "external only",
			external: []internal.MirrorResult{mirror("a.srs", "b.srs"), mirror("c.srs")},
			want:     "update: external (3 files)",
		},
		{
			name:    "domains and custom",
			domains: []string{"github"},
			custom:  custom(2),
			want:    "update: github (1 domains), custom (2 rules)",
		},
		{
			name:     "all three",
			domains:  []string{"github"},
			custom:   custom(1),
			external: []internal.MirrorResult{mirror("a.srs")},
			want:     "update: github (1 domains), custom (1 rules), external (1 files)",
		},
		{
			name:     "external sources with zero files are ignored",
			external: []internal.MirrorResult{mirror()},
			want:     "update: (no changes)",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildCommitMessage(tc.domains, tc.custom, tc.external)
			if got != tc.want {
				t.Errorf("buildCommitMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
