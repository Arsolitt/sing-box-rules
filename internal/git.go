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
	s := strings.TrimSpace(string(out))
	if err != nil {
		return s, fmt.Errorf("git %s: %s", strings.Join(args, " "), s)
	}
	return s, nil
}

func (g *GitClient) RunGit(args ...string) (string, error) {
	return g.runGit(args...)
}

func (g *GitClient) CheckoutRuleSetBranch() error {
	if _, err := g.runGit("init"); err != nil {
		return err
	}

	g.runGit("config", "user.email", "sing-box-rules[bot]@users.noreply")
	g.runGit("config", "user.name", "sing-box-rules[bot]")

	originURL := g.getOriginURL()
	if originURL != "" {
		g.runGit("remote", "add", "origin", originURL)
	}

	if _, err := g.runGit("fetch", "origin", "rule-set"); err == nil {
		if _, err := g.runGit("checkout", "rule-set"); err != nil {
			return err
		}
		return nil
	}

	if _, err := g.runGit("checkout", "--orphan", "rule-set"); err != nil {
		return err
	}

	return nil
}

func (g *GitClient) getOriginURL() string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = g.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (g *GitClient) StageAll() error {
	_, err := g.runGit("add", "-A")
	return err
}

func (g *GitClient) HasChanges() (bool, error) {
	out, err := g.runGit("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (g *GitClient) Commit(message string) error {
	_, err := g.runGit("commit", "--allow-empty", "-m", message)
	return err
}

func (g *GitClient) Push() error {
	_, err := g.runGit("push", "origin", "rule-set")
	return err
}

func (g *GitClient) PullRebase() error {
	_, err := g.runGit("pull", "--rebase", "origin", "rule-set")
	return err
}
