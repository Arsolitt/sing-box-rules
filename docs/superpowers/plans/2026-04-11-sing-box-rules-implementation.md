# sing-box-rules Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go program that fetches IP ranges from ipinfo API, compiles sing-box rule-set (.srs) files, and publishes them to a `rule-set` git branch via GitHub Actions.

**Architecture:** Monolithic Go binary with internal packages for config loading, API fetching, JSON-to-rule-set transformation, .srs compilation, and git operations. Runs daily via GitHub Actions cron.

**Tech Stack:** Go 1.22+, sing-box v1.13.7 (library), GitHub Actions

---

## File Structure

```
sing-box-rules/
├── .github/workflows/update.yml    # daily cron + manual dispatch
├── cmd/sing-box-rules/main.go      # entry point, orchestrates all steps
├── internal/
│   ├── config.go                   # DomainConfig struct, LoadConfig()
│   ├── config_test.go              # tests for config loading
│   ├── fetcher.go                  # FetchRanges() — HTTP to ipinfo
│   ├── fetcher_test.go             # tests for fetcher (httptest)
│   ├── transformer.go              # Transform() — ipinfo JSON → PlainRuleSet
│   ├── transformer_test.go         # tests for transformation
│   ├── compiler.go                 # Compile() — PlainRuleSet → .srs bytes
│   ├── compiler_test.go            # tests for SRS compilation
│   ├── custom.go                   # CompileCustomRules() — JSON files → .srs
│   ├── custom_test.go              # tests for custom rules
│   ├── git.go                      # GitClient — branch checkout, commit, push
│   ├── git_test.go                 # tests for git operations
│   ├── scheduler.go                # DetermineOutdated() — filter by git history
│   └── scheduler_test.go           # tests for scheduling logic
├── config/domains.json             # domain configuration
├── custom-rules/                   # (empty, for future manual rules)
└── go.mod
```

---

### Task 1: Project scaffold and go.mod

**Files:**
- Create: `go.mod`
- Create: `cmd/sing-box-rules/main.go` (minimal placeholder)
- Create: `internal/config.go` (minimal placeholder)

- [ ] **Step 1: Initialize Go module and add sing-box dependency**

```bash
cd /home/arsolitt/projects/sing-box-rules
go mod init github.com/arsolitt/sing-box-rules
go get github.com/sagernet/sing-box@v1.13.7
```

Expected: `go.mod` created with sing-box v1.13.7 and its transitive dependencies.

- [ ] **Step 2: Create minimal main.go placeholder**

```go
package main

import "fmt"

func main() {
	fmt.Println("sing-box-rules")
}
```

Save to `cmd/sing-box-rules/main.go`.

- [ ] **Step 3: Verify it compiles**

Run: `go build ./cmd/sing-box-rules`
Expected: builds without errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum cmd/sing-box-rules/main.go
git commit -m "feat: init Go module with sing-box v1.13.7 dependency"
```

---

### Task 2: Config loading

**Files:**
- Create: `internal/config.go`
- Create: `internal/config_test.go`
- Create: `config/domains.json`

- [ ] **Step 1: Write the failing test**

```go
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
```

Save to `internal/config_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestLoadConfig -v`
Expected: FAIL — `LoadConfig` not defined.

- [ ] **Step 3: Write implementation**

```go
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
```

Save to `internal/config.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run TestLoadConfig -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Create config/domains.json**

```json
[
  {
    "name": "github",
    "domain": "github.com",
    "interval_days": 30,
    "extra_domains": ["github.io", "ghcr.io", "github.githubassets.com", "githubusercontent.com"]
  },
  {
    "name": "amazon",
    "domain": "amazon.com",
    "interval_days": 7,
    "extra_domains": ["amazonaws.com", "cloudfront.net"]
  },
  {
    "name": "cloudflare",
    "domain": "cloudflare.com",
    "interval_days": 7,
    "extra_domains": []
  },
  {
    "name": "microsoft",
    "domain": "microsoft.com",
    "interval_days": 30,
    "extra_domains": ["azure.com", "windows.net", "msn.com"]
  }
]
```

Save to `config/domains.json`.

- [ ] **Step 6: Commit**

```bash
git add internal/config.go internal/config_test.go config/domains.json
git commit -m "feat: add config loading with domain definitions"
```

---

### Task 3: Fetcher — ipinfo API client

**Files:**
- Create: `internal/fetcher.go`
- Create: `internal/fetcher_test.go`

- [ ] **Step 1: Write the failing test**

```go
package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRanges(t *testing.T) {
	response := IPInfoResponse{
		Domain:    "github.com",
		RedirectsTo: nil,
		NumRanges: 2,
		Ranges:    []string{"1.2.3.0/24", "2401:cf20::/32"},
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
```

Save to `internal/fetcher_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestFetchRanges -v`
Expected: FAIL — `FetchRanges`, `IPInfoResponse`, `IsRateLimitError` not defined.

- [ ] **Step 3: Write implementation**

```go
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
```

Save to `internal/fetcher.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run TestFetchRanges -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/fetcher.go internal/fetcher_test.go
git commit -m "feat: add ipinfo API fetcher with rate limit detection"
```

---

### Task 4: Transformer — ipinfo JSON → sing-box PlainRuleSet

**Files:**
- Create: `internal/transformer.go`
- Create: `internal/transformer_test.go`

- [ ] **Step 1: Write the failing test**

```go
package internal

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestTransform(t *testing.T) {
	ipinfo := &IPInfoResponse{
		Domain: "github.com",
		Ranges: []string{"1.2.3.0/24", "2401:cf20::/32"},
	}
	extraDomains := []string{"github.io", "ghcr.io"}

	ruleSet := Transform(ipinfo, extraDomains)

	if len(ruleSet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ruleSet.Rules))
	}

	rule := ruleSet.Rules[0]
	if rule.Type != C.RuleTypeDefault {
		t.Errorf("expected type 'default', got %q", rule.Type)
	}

	domainSuffix := []string(rule.DefaultOptions.DomainSuffix)
	if len(domainSuffix) != 3 {
		t.Errorf("expected 3 domain_suffix entries, got %d: %v", len(domainSuffix), domainSuffix)
	}

	expectedDomains := map[string]bool{
		"github.com":  false,
		"github.io":   false,
		"ghcr.io":     false,
	}
	for _, d := range domainSuffix {
		if _, ok := expectedDomains[d]; !ok {
			t.Errorf("unexpected domain_suffix: %q", d)
		}
		expectedDomains[d] = true
	}
	for d, found := range expectedDomains {
		if !found {
			t.Errorf("missing domain_suffix: %q", d)
		}
	}

	ipCIDR := []string(rule.DefaultOptions.IPCIDR)
	if len(ipCIDR) != 2 {
		t.Errorf("expected 2 ip_cidr entries, got %d: %v", len(ipCIDR), ipCIDR)
	}
}

func TestTransformNoExtraDomains(t *testing.T) {
	ipinfo := &IPInfoResponse{
		Domain: "cloudflare.com",
		Ranges: []string{"1.1.1.0/24"},
	}

	ruleSet := Transform(ipinfo, nil)

	domainSuffix := []string(rule.DefaultOptions.DomainSuffix)
	if len(domainSuffix) != 1 || domainSuffix[0] != "cloudflare.com" {
		t.Errorf("expected ['cloudflare.com'], got %v", domainSuffix)
	}
}

func TestTransformEmptyRanges(t *testing.T) {
	ipinfo := &IPInfoResponse{
		Domain: "example.com",
		Ranges: []string{},
	}

	ruleSet := Transform(ipinfo, nil)

	ipCIDR := []string(rule.DefaultOptions.IPCIDR)
	if len(ipCIDR) != 0 {
		t.Errorf("expected 0 ip_cidr entries, got %d", len(ipCIDR))
	}
}

func TestRuleSetToPlainRuleSet(t *testing.T) {
	ruleSet := option.PlainRuleSet{
		Rules: []option.HeadlessRule{
			{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					DomainSuffix: []string{"example.com"},
					IPCIDR:       []string{"1.2.3.0/24"},
				},
			},
		},
	}

	if len(ruleSet.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ruleSet.Rules))
	}
}
```

Save to `internal/transformer_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestTransform -v`
Expected: FAIL — `Transform` not defined.

- [ ] **Step 3: Write implementation**

```go
package internal

import (
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func Transform(ipinfo *IPInfoResponse, extraDomains []string) option.PlainRuleSet {
	domainSuffix := make([]string, 0, 1+len(extraDomains))
	domainSuffix = append(domainSuffix, ipinfo.Domain)
	domainSuffix = append(domainSuffix, extraDomains...)

	ipCIDR := make([]string, len(ipinfo.Ranges))
	copy(ipCIDR, ipinfo.Ranges)

	return option.PlainRuleSet{
		Rules: []option.HeadlessRule{
			{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					DomainSuffix: domainSuffix,
					IPCIDR:       ipCIDR,
				},
			},
		},
	}
}
```

Save to `internal/transformer.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run TestTransform -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/transformer.go internal/transformer_test.go
git commit -m "feat: add transformer from ipinfo response to sing-box PlainRuleSet"
```

---

### Task 5: Compiler — PlainRuleSet → .srs bytes

**Files:**
- Create: `internal/compiler.go`
- Create: `internal/compiler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package internal

import (
	"bytes"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
)

func TestCompile(t *testing.T) {
	ruleSet := option.PlainRuleSet{
		Rules: []option.HeadlessRule{
			{
				Type: C.RuleTypeDefault,
				DefaultOptions: option.DefaultHeadlessRule{
					DomainSuffix: []string{"example.com", "www.example.com"},
					IPCIDR:       []string{"1.2.3.0/24"},
				},
			},
		},
	}

	srsData, err := Compile(ruleSet)
	if err != nil {
		t.Fatal(err)
	}

	if len(srsData) == 0 {
		t.Fatal("expected non-empty .srs data")
	}

	if srsData[0] != 0x53 || srsData[1] != 0x52 || srsData[2] != 0x53 {
		t.Errorf("expected SRS magic bytes at start, got %x", srsData[:3])
	}

	reader := bytes.NewReader(srsData)
	_, err = srs.Read(reader, true)
	if err != nil {
		t.Fatalf("failed to read back compiled .srs: %v", err)
	}
}

func TestCompileEmptyRules(t *testing.T) {
	ruleSet := option.PlainRuleSet{
		Rules: []option.HeadlessRule{},
	}

	srsData, err := Compile(ruleSet)
	if err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewReader(srsData)
	_, err = srs.Read(reader, true)
	if err != nil {
		t.Fatalf("failed to read back empty .srs: %v", err)
	}
}
```

Save to `internal/compiler_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestCompile -v`
Expected: FAIL — `Compile` not defined.

- [ ] **Step 3: Write implementation**

```go
package internal

import (
	"bytes"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/common/srs"
	"github.com/sagernet/sing-box/option"
)

func Compile(ruleSet option.PlainRuleSet) ([]byte, error) {
	var buf bytes.Buffer
	if err := srs.Write(&buf, ruleSet, C.RuleSetVersionCurrent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
```

Save to `internal/compiler.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run TestCompile -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/compiler.go internal/compiler_test.go
git commit -m "feat: add SRS compiler using sing-box srs.Write"
```

---

### Task 6: Custom rules compilation

**Files:**
- Create: `internal/custom.go`
- Create: `internal/custom_test.go`

- [ ] **Step 1: Write the failing test**

```go
package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func TestCompileCustomRules(t *testing.T) {
	dir := t.TempDir()

	ruleSet := map[string]interface{}{
		"rules": []map[string]interface{}{
			{
				"domain_suffix": []string{"example.com"},
				"ip_cidr":       []string{"10.0.0.0/8"},
			},
		},
		"version": 3,
	}
	data, _ := json.MarshalIndent(ruleSet, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "my-rule.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	results, err := CompileCustomRules(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "my-rule.srs" {
		t.Errorf("expected name 'my-rule.srs', got %q", results[0].Name)
	}
	if len(results[0].Data) == 0 {
		t.Error("expected non-empty .srs data")
	}
}

func TestCompileCustomRulesEmptyDir(t *testing.T) {
	dir := t.TempDir()

	results, err := CompileCustomRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty dir, got %d", len(results))
	}
}

func TestCompileCustomRulesSkipsNonJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)

	results, err := CompileCustomRules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestCompileCustomRulesInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not valid json {{{"), 0644)

	_, err := CompileCustomRules(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCustomRuleSetJSONParsing(t *testing.T) {
	jsonData := `{
		"rules": [
			{
				"domain_suffix": ["example.com"],
				"ip_cidr": ["10.0.0.0/8"]
			}
		],
		"version": 3
	}`

	var compat option.PlainRuleSetCompat
	if err := json.Unmarshal([]byte(jsonData), &compat); err != nil {
		t.Fatal(err)
	}
	if compat.Version != C.RuleSetVersion3 {
		t.Errorf("expected version 3, got %d", compat.Version)
	}
	if len(compat.Options.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(compat.Options.Rules))
	}
}
```

Save to `internal/custom_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestCompileCustomRules -v`
Expected: FAIL — `CompileCustomRules`, `CustomResult` not defined.

- [ ] **Step 3: Write implementation**

```go
package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagernet/sing-box/option"
)

type CustomResult struct {
	Name string
	Data []byte
}

func CompileCustomRules(dir string) ([]CustomResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read custom-rules dir: %w", err)
	}

	var results []CustomResult
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		var compat option.PlainRuleSetCompat
		if err := json.Unmarshal(data, &compat); err != nil {
			return nil, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}

		ruleSet, err := compat.Upgrade()
		if err != nil {
			return nil, fmt.Errorf("upgrade %s: %w", entry.Name(), err)
		}

		srsData, err := Compile(ruleSet)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", entry.Name(), err)
		}

		baseName := strings.TrimSuffix(entry.Name(), ".json")
		results = append(results, CustomResult{
			Name: baseName + ".srs",
			Data: srsData,
		})
	}

	return results, nil
}
```

Save to `internal/custom.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run "TestCompileCustomRules|TestCustomRuleSetJSONParsing" -v`
Expected: PASS (5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/custom.go internal/custom_test.go
git commit -m "feat: add custom rules compilation from JSON to .srs"
```

---

### Task 7: Git client

**Files:**
- Create: `internal/git.go`
- Create: `internal/git_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

Save to `internal/git_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestGitClient -v`
Expected: FAIL — `NewGitClient`, `GitClient` not defined.

- [ ] **Step 3: Write implementation**

```go
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
```

Save to `internal/git.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run TestGitClient -v`
Expected: PASS (6 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/git.go internal/git_test.go
git commit -m "feat: add git client for rule-set branch operations"
```

---

### Task 8: Scheduler — determine outdated domains via git history

**Files:**
- Create: `internal/scheduler.go`
- Create: `internal/scheduler_test.go`

- [ ] **Step 1: Write the failing test**

```go
package internal

import (
	"os"
	"os/exec"
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
```

Save to `internal/scheduler_test.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ -run TestDetermineOutdated -v`
Expected: FAIL — `DetermineOutdated` not defined.

- [ ] **Step 3: Write implementation**

```go
package internal

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"os/exec"
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
	out, err := client.RunGit("log", "--format=%ct", "--follow", "--", filename)
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

func (g *GitClient) RunGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
```

Save to `internal/scheduler.go`.

Note: The `RunGit` method is exported (uppercase) so `scheduler.go` can call it. The `runGit` method in `git.go` should be renamed to `RunGit` or `scheduler.go`'s `getLastCommitDate` should use a different approach. Since `git.go` already has a private `runGit`, add a public `RunGit` method to `git.go` that wraps the private one. Update `git.go` to export:

In `internal/git.go`, add after the existing `runGit` method:

```go
func (g *GitClient) RunGit(args ...string) (string, error) {
	return g.runGit(args...)
}
```

And remove the `RunGit` method from `internal/scheduler.go` — it should only exist in `git.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/ -run "TestDetermineOutdated|TestSortByInterval" -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/scheduler.go internal/scheduler_test.go internal/git.go
git commit -m "feat: add scheduler for determining outdated domains via git history"
```

---

### Task 9: Main entry point — orchestrate the full flow

**Files:**
- Modify: `cmd/sing-box-rules/main.go`

- [ ] **Step 1: Write the main.go implementation**

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/arsolitt/sing-box-rules/internal"
)

const ipinfoBaseURL = "https://ipinfo.io"

func main() {
	configPath := flag.String("config", "config/domains.json", "path to domains config")
	customRulesDir := flag.String("custom-rules", "custom-rules", "path to custom rules directory")
	workDir := flag.String("work-dir", "", "working directory for git operations")
	flag.Parse()

	configs, err := internal.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if *workDir == "" {
		*workDir, err = os.MkdirTemp("", "sing-box-rules-*")
		if err != nil {
			log.Fatalf("create temp dir: %v", err)
		}
		defer os.RemoveAll(*workDir)
	}

	repoDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("get working directory: %v", err)
	}

	client := internal.NewGitClient(repoDir, *workDir)

	log.Println("checking out rule-set branch...")
	if err := client.CheckoutRuleSetBranch(); err != nil {
		log.Fatalf("checkout rule-set branch: %v", err)
	}

	log.Println("determining outdated domains...")
	outdated, err := internal.DetermineOutdated(client, configs)
	if err != nil {
		log.Fatalf("determine outdated: %v", err)
	}

	log.Printf("found %d outdated domains", len(outdated))

	var updatedDomains []string

	for _, cfg := range outdated {
		log.Printf("fetching ranges for %s...", cfg.Domain)
		resp, err := internal.FetchRanges(ipinfoBaseURL, cfg.Domain)
		if err != nil {
			if internal.IsRateLimitError(err) {
				log.Printf("rate limited, stopping. %d domains updated this run.", len(updatedDomains))
				break
			}
			log.Printf("error fetching %s: %v, stopping.", cfg.Domain, err)
			break
		}

		ruleSet := internal.Transform(resp, cfg.ExtraDomains)
		srsData, err := internal.Compile(ruleSet)
		if err != nil {
			log.Printf("error compiling %s: %v, skipping.", cfg.Name, err)
			continue
		}

	 outputPath := filepath.Join(*workDir, cfg.Name+".srs")
		if err := os.WriteFile(outputPath, srsData, 0644); err != nil {
			log.Printf("error writing %s: %v, skipping.", cfg.Name, err)
			continue
		}

		updatedDomains = append(updatedDomains, cfg.Name)
		log.Printf("updated %s.srs (%d ranges)", cfg.Name, len(resp.Ranges))
	}

	log.Println("compiling custom rules...")
	customResults, err := internal.CompileCustomRules(*customRulesDir)
	if err != nil {
		log.Printf("warning: custom rules error: %v", err)
	} else {
		for _, cr := range customResults {
			outputPath := filepath.Join(*workDir, cr.Name)
			if err := os.WriteFile(outputPath, cr.Data, 0644); err != nil {
				log.Printf("error writing custom rule %s: %v", cr.Name, err)
				continue
			}
			log.Printf("compiled custom rule: %s", cr.Name)
		}
	}

	if err := client.StageAll(); err != nil {
		log.Fatalf("git stage: %v", err)
	}

	hasChanges, err := client.HasChanges()
	if err != nil {
		log.Fatalf("git status: %v", err)
	}

	if !hasChanges {
		log.Println("no changes to commit")
		return
	}

	sort.Strings(updatedDomains)
	commitMsg := buildCommitMessage(updatedDomains, customResults)
	log.Printf("committing: %s", commitMsg)

	if err := client.Commit(commitMsg); err != nil {
		log.Fatalf("git commit: %v", err)
	}

	log.Println("pulling with rebase before push...")
	if err := client.PullRebase(); err != nil {
		log.Printf("warning: pull rebase failed: %v", err)
	}

	log.Println("pushing to rule-set branch...")
	if err := client.Push(); err != nil {
		log.Fatalf("git push: %v", err)
	}

	log.Println("done!")
}

func buildCommitMessage(domains []string, customResults []internal.CustomResult) string {
	if len(domains) > 0 && len(customResults) > 0 {
		return fmt.Sprintf("update: %s, custom (%d rules)", strings.Join(domains, ", "), len(customResults))
	}
	if len(domains) > 0 {
		return fmt.Sprintf("update: %s (%d domains)", strings.Join(domains, ", "), len(domains))
	}
	if len(customResults) > 0 {
		return fmt.Sprintf("update: custom (%d rules)", len(customResults))
	}
	return "update: (no changes)"
}
```

Save to `cmd/sing-box-rules/main.go`.

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/sing-box-rules`
Expected: builds without errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/sing-box-rules/main.go
git commit -m "feat: add main entry point orchestrating full update flow"
```

---

### Task 10: GitHub Actions workflow

**Files:**
- Create: `.github/workflows/update.yml`

- [ ] **Step 1: Create the workflow file**

```yaml
name: Update Rule Sets

on:
  schedule:
    - cron: '0 8 * * *'
  workflow_dispatch:

permissions:
  contents: write

jobs:
  update:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
          token: ${{ secrets.GITHUB_TOKEN }}

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Update rule sets
        run: go run ./cmd/sing-box-rules
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Save to `.github/workflows/update.yml`.

- [ ] **Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/update.yml'))"`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/update.yml
git commit -m "ci: add daily cron workflow for rule-set updates"
```

---

### Task 11: Integration test — dry run with real ipinfo API

**Files:**
- None (manual verification)

- [ ] **Step 1: Run the full flow locally (dry run, no git push)**

Run: `go run ./cmd/sing-box-rules -work-dir /tmp/sing-box-rules-test`
Expected: fetches ranges from ipinfo for all domains, compiles .srs files, attempts git operations (will fail on push since no remote is configured — that's OK for local testing).

- [ ] **Step 2: Verify .srs files are created in work dir**

Run: `ls -la /tmp/sing-box-rules-test/*.srs`
Expected: multiple .srs files present.

- [ ] **Step 3: Verify .srs files are valid**

Run: `go run -exec '' ./cmd/sing-box-rules` and check output for any compilation errors. Or manually verify one .srs file by reading its first 3 bytes (should be `SRS` magic).

---

### Task 12: Run all tests

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: all tests pass.

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 3: Commit any remaining fixes if needed**

```bash
git add -A
git commit -m "fix: address test and vet issues"
```

---

## Self-Review Checklist

**1. Spec coverage:**
- [x] Go program with sing-box v1.13.7 as library — Task 1
- [x] Config loading from `config/domains.json` — Task 2
- [x] ipinfo API fetching with rate limit detection — Task 3
- [x] Data transformation (ipinfo → PlainRuleSet) — Task 4
- [x] SRS compilation — Task 5
- [x] Custom rules compilation — Task 6
- [x] Git client (branch checkout, commit, push) — Task 7
- [x] Scheduler (determine outdated via git history) — Task 8
- [x] Main orchestration — Task 9
- [x] GitHub Actions workflow — Task 10
- [x] Greedy strategy with break on 429/error — Task 9 main.go
- [x] Sort by interval_days asc — Task 8
- [x] Files without commits = always outdated — Task 8
- [x] Custom rules compiled every run, pushed only if changed — Task 9
- [x] `git pull --rebase` before push — Task 9
- [x] Atomic commits — Task 9 buildCommitMessage

**2. Placeholder scan:**
- No TBD/TODO items found
- All code blocks contain actual code
- All commands have expected outputs

**3. Type consistency:**
- `DomainConfig` defined once in config.go, used everywhere
- `IPInfoResponse` defined once in fetcher.go, used in transformer.go
- `PlainRuleSet` from sing-box option package, used consistently
- `badoption.Listable` is `[]T` — direct slice assignment works
- `RuleSetVersionCurrent` = 4 (from constant package)
- `RuleTypeDefault` = "default" (from constant package)
