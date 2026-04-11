package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) (repoDir, workDir string) {
	t.Helper()
	repoDir = t.TempDir()
	workDir = t.TempDir()

	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "config", "user.name", "Test")

	initialFile := filepath.Join(repoDir, "README.md")
	os.WriteFile(initialFile, []byte("# test"), 0644)
	runGit(t, repoDir, "add", ".")
	runGit(t, repoDir, "commit", "-m", "init")

	return
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %s\n%s", args, err, string(out))
	}
}

func TestGitClientCheckoutRuleSetBranch(t *testing.T) {
	repoDir, workDir := initTestRepo(t)

	client := NewGitClient(repoDir, workDir)
	err := client.CheckoutRuleSetBranch()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = workDir
	out, _ := cmd.Output()
	if string(out) != "rule-set\n" {
		t.Errorf("expected branch 'rule-set', got %q", string(out))
	}
}

func TestGitClientCheckoutRuleSetBranchExisting(t *testing.T) {
	repoDir, workDir := initTestRepo(t)

	runGit(t, repoDir, "checkout", "-b", "rule-set")
	runGit(t, repoDir, "checkout", "main")

	client := NewGitClient(repoDir, workDir)
	err := client.CheckoutRuleSetBranch()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = workDir
	out, _ := cmd.Output()
	if string(out) != "rule-set\n" {
		t.Errorf("expected branch 'rule-set', got %q", string(out))
	}
}

func TestGitClientHasChanges(t *testing.T) {
	repoDir, workDir := initTestRepo(t)
	client := NewGitClient(repoDir, workDir)
	client.CheckoutRuleSetBranch()

	hasChanges, err := client.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if hasChanges {
		t.Error("expected no changes in clean repo")
	}

	newFile := filepath.Join(workDir, "test.srs")
	os.WriteFile(newFile, []byte("data"), 0644)

	hasChanges, err = client.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !hasChanges {
		t.Error("expected changes after adding file")
	}
}

func TestGitClientCommitAndPush(t *testing.T) {
	repoDir, workDir := initTestRepo(t)
	client := NewGitClient(repoDir, workDir)
	client.CheckoutRuleSetBranch()

	newFile := filepath.Join(workDir, "test.srs")
	os.WriteFile(newFile, []byte("data"), 0644)

	err := client.Commit("update: test (1 domain)")
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "log", "--oneline", "-1")
	cmd.Dir = workDir
	out, _ := cmd.Output()
	if string(out) == "" {
		t.Error("expected a commit to exist")
	}
}

func TestGitClientCommitNoChanges(t *testing.T) {
	repoDir, workDir := initTestRepo(t)
	client := NewGitClient(repoDir, workDir)
	client.CheckoutRuleSetBranch()

	err := client.Commit("update: nothing")
	if err != nil {
		t.Fatal(err)
	}
}

func TestGitClientStageAll(t *testing.T) {
	repoDir, workDir := initTestRepo(t)
	client := NewGitClient(repoDir, workDir)
	client.CheckoutRuleSetBranch()

	newFile := filepath.Join(workDir, "untracked.srs")
	os.WriteFile(newFile, []byte("data"), 0644)

	err := client.StageAll()
	if err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	cmd.Dir = workDir
	out, _ := cmd.Output()
	if string(out) != "untracked.srs\n" {
		t.Errorf("expected 'untracked.srs' staged, got %q", string(out))
	}
}
