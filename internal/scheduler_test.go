package internal

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestDetermineOutdated(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	runGit(t, repoDir, "checkout", "--orphan", "rule-set")

	configs := []DomainConfig{
		{Name: "github", Domain: "github.com", IntervalDays: 30, ExtraDomains: nil},
		{Name: "amazon", Domain: "amazon.com", IntervalDays: 7, ExtraDomains: nil},
		{Name: "cloudflare", Domain: "cloudflare.com", IntervalDays: 7, ExtraDomains: nil},
	}

	client := NewGitClient(repoDir, workDir)
	client.CheckoutRuleSetBranch()

	outdated, err := DetermineOutdated(client, configs)
	if err != nil {
		t.Fatal(err)
	}

	if len(outdated) != 3 {
		t.Fatalf("expected all 3 domains to be outdated (no prior commits), got %d", len(outdated))
	}

	names := make([]string, len(outdated))
	for i, dc := range outdated {
		names[i] = dc.Name
	}

	amazonIdx := -1
	githubIdx := -1
	for i, name := range names {
		if name == "amazon" {
			amazonIdx = i
		}
		if name == "github" {
			githubIdx = i
		}
	}

	if amazonIdx >= githubIdx {
		t.Error("amazon (interval 7) should come before github (interval 30) in sort order")
	}
}

func TestDetermineOutdatedWithRecentCommit(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	runGit(t, repoDir, "checkout", "--orphan", "rule-set")

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.email", "test@example.com")
	runGit(t, workDir, "config", "user.name", "Test")

	os.WriteFile(filepath.Join(workDir, "github.srs"), []byte("data"), 0644)
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "update: github")

	configs := []DomainConfig{
		{Name: "github", Domain: "github.com", IntervalDays: 30, ExtraDomains: nil},
	}

	client := NewGitClient(repoDir, workDir)

	outdated, err := DetermineOutdated(client, configs)
	if err != nil {
		t.Fatal(err)
	}

	if len(outdated) != 0 {
		t.Errorf("github was just committed, should not be outdated, got %d outdated", len(outdated))
	}
}

func TestDetermineOutdatedExpiredCommit(t *testing.T) {
	repoDir := t.TempDir()
	workDir := t.TempDir()

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("# test"), 0644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	runGit(t, repoDir, "checkout", "--orphan", "rule-set")

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.email", "test@example.com")
	runGit(t, workDir, "config", "user.name", "Test")

	os.WriteFile(filepath.Join(workDir, "github.srs"), []byte("data"), 0644)
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "update: github")

	pastDate := time.Now().AddDate(0, 0, -31).Format("2006-01-02T15:04:05")
	runGit(t, workDir, "commit", "--amend", "--date", pastDate, "--no-edit")

	configs := []DomainConfig{
		{Name: "github", Domain: "github.com", IntervalDays: 30, ExtraDomains: nil},
	}

	client := NewGitClient(repoDir, workDir)

	outdated, err := DetermineOutdated(client, configs)
	if err != nil {
		t.Fatal(err)
	}

	if len(outdated) != 1 {
		t.Errorf("github commit is 31 days old with interval 30, should be outdated, got %d", len(outdated))
	}
}

func TestSortByInterval(t *testing.T) {
	configs := []DomainConfig{
		{Name: "a", Domain: "a.com", IntervalDays: 30},
		{Name: "b", Domain: "b.com", IntervalDays: 7},
		{Name: "c", Domain: "c.com", IntervalDays: 14},
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].IntervalDays < configs[j].IntervalDays
	})
	if configs[0].Name != "b" || configs[1].Name != "c" || configs[2].Name != "a" {
		t.Errorf("sort failed: %v", configs)
	}
}
