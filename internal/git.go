package internal

import (
	"fmt"
	"os/exec"
	"strings"
)

type GitClient struct {
	repoDir string
	workDir string
}

func NewGitClient(repoDir, workDir string) *GitClient {
	return &GitClient{repoDir: repoDir, workDir: workDir}
}

func (g *GitClient) runGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func (g *GitClient) RunGit(args ...string) (string, error) {
	return g.runGit(args...)
}

func (g *GitClient) CheckoutRuleSetBranch() error {
	_, err := g.runGit("init")
	if err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	g.runGit("config", "user.email", "sing-box-rules[bot]@users.noreply")
	g.runGit("config", "user.name", "sing-box-rules[bot]")

	_, err = g.runGit("fetch", "origin", "rule-set")
	if err == nil {
		_, err = g.runGit("checkout", "rule-set")
		if err != nil {
			return fmt.Errorf("checkout rule-set: %w", err)
		}
		return nil
	}

	_, err = g.runGit("checkout", "--orphan", "rule-set")
	if err != nil {
		return fmt.Errorf("create orphan branch: %w", err)
	}

	return nil
}

func (g *GitClient) StageAll() error {
	_, err := g.runGit("add", "-A")
	if err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	return nil
}

func (g *GitClient) HasChanges() (bool, error) {
	out, err := g.runGit("status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return out != "", nil
}

func (g *GitClient) Commit(message string) error {
	_, err := g.runGit("commit", "--allow-empty", "-m", message)
	if err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

func (g *GitClient) Push() error {
	_, err := g.runGit("push", "origin", "rule-set")
	if err != nil {
		return fmt.Errorf("git push: %w", err)
	}
	return nil
}

func (g *GitClient) PullRebase() error {
	_, err := g.runGit("pull", "--rebase", "origin", "rule-set")
	if err != nil {
		return fmt.Errorf("git pull rebase: %w", err)
	}
	return nil
}
