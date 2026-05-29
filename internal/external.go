package internal

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultCodeloadBaseURL  = "https://codeload.github.com"
	defaultGitHubAPIBaseURL = "https://api.github.com"
	srsSuffix               = ".srs"
)

// ExternalSource describes a third-party rule-set repository to mirror as-is.
// All .srs files from the source are copied into the rule-set branch, each
// renamed with the configured Prefix.
type ExternalSource struct {
	Name   string `json:"name"`             // logical name, used in logs and commit messages
	Type   string `json:"type"`             // "branch" or "release"
	Repo   string `json:"repo"`             // owner/repo on GitHub
	Branch string `json:"branch,omitempty"` // branch name (type "branch")
	Tag    string `json:"tag,omitempty"`    // release tag (type "release"); empty means latest
	Prefix string `json:"prefix,omitempty"` // prepended to every mirrored filename
}

// MirrorResult reports the files written for a single source.
type MirrorResult struct {
	Source string
	Files  []string
}

// LoadExternalConfig reads the external sources config file.
func LoadExternalConfig(path string) ([]ExternalSource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read external config: %w", err)
	}

	var sources []ExternalSource
	if err := json.Unmarshal(data, &sources); err != nil {
		return nil, fmt.Errorf("parse external config: %w", err)
	}

	return sources, nil
}

// IsValidSRS reports whether data starts with the sing-box rule-set magic
// header ("SRS" followed by a version byte). It guards against committing
// HTML error pages or truncated downloads as if they were rule-sets.
func IsValidSRS(data []byte) bool {
	return len(data) >= 4 && data[0] == 'S' && data[1] == 'R' && data[2] == 'S'
}

func httpClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}

func authHeader(req *http.Request) {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// MirrorExternal dispatches to the appropriate mirror strategy for the source
// type, using production GitHub endpoints.
func MirrorExternal(src ExternalSource, destDir string) (MirrorResult, error) {
	switch src.Type {
	case "branch":
		return MirrorBranch(src, destDir, defaultCodeloadBaseURL)
	case "release":
		return MirrorRelease(src, destDir, defaultGitHubAPIBaseURL)
	default:
		return MirrorResult{}, fmt.Errorf("unknown external source type %q for %s", src.Type, src.Name)
	}
}

// MirrorBranch downloads the branch tarball and extracts every .srs file into
// destDir, prefixing each filename. codeloadBaseURL is the codeload host
// (parameterized for testing).
func MirrorBranch(src ExternalSource, destDir, codeloadBaseURL string) (MirrorResult, error) {
	url := fmt.Sprintf("%s/%s/tar.gz/refs/heads/%s", strings.TrimRight(codeloadBaseURL, "/"), src.Repo, src.Branch)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return MirrorResult{}, fmt.Errorf("build request for %s: %w", src.Name, err)
	}
	authHeader(req)

	resp, err := httpClient().Do(req)
	if err != nil {
		return MirrorResult{}, fmt.Errorf("download tarball for %s: %w", src.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return MirrorResult{}, fmt.Errorf("tarball for %s returned HTTP %d: %s", src.Name, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	files, err := extractSRSFromTarball(resp.Body, destDir, src.Prefix)
	if err != nil {
		return MirrorResult{}, fmt.Errorf("extract tarball for %s: %w", src.Name, err)
	}

	return MirrorResult{Source: src.Name, Files: files}, nil
}

// extractSRSFromTarball reads a gzip-compressed tar stream and writes every
// regular *.srs entry into destDir as prefix+basename. Entries are reduced to
// their basename (protecting against path traversal) and validated against the
// SRS magic header; invalid entries are skipped.
func extractSRSFromTarball(r io.Reader, destDir, prefix string) ([]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var written []string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if !strings.HasSuffix(hdr.Name, srsSuffix) {
			continue
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read tar content for %s: %w", hdr.Name, err)
		}
		if !IsValidSRS(data) {
			continue
		}

		outName := prefix + filepath.Base(hdr.Name)
		if err := os.WriteFile(filepath.Join(destDir, outName), data, 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", outName, err)
		}
		written = append(written, outName)
	}

	sort.Strings(written)
	return written, nil
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseResponse struct {
	Assets []releaseAsset `json:"assets"`
}

// MirrorRelease fetches a GitHub release and downloads every .srs asset into
// destDir, prefixing each filename. apiBaseURL is the GitHub API host
// (parameterized for testing). An empty src.Tag selects the latest release.
func MirrorRelease(src ExternalSource, destDir, apiBaseURL string) (MirrorResult, error) {
	base := strings.TrimRight(apiBaseURL, "/")
	var apiURL string
	if src.Tag == "" {
		apiURL = fmt.Sprintf("%s/repos/%s/releases/latest", base, src.Repo)
	} else {
		apiURL = fmt.Sprintf("%s/repos/%s/releases/tags/%s", base, src.Repo, src.Tag)
	}

	release, err := fetchRelease(apiURL)
	if err != nil {
		return MirrorResult{}, fmt.Errorf("fetch release for %s: %w", src.Name, err)
	}

	client := httpClient()
	var written []string
	for _, asset := range release.Assets {
		if !strings.HasSuffix(asset.Name, srsSuffix) {
			continue
		}

		data, err := downloadAsset(client, asset.BrowserDownloadURL)
		if err != nil {
			return MirrorResult{}, fmt.Errorf("download asset %s for %s: %w", asset.Name, src.Name, err)
		}
		if !IsValidSRS(data) {
			continue
		}

		outName := src.Prefix + asset.Name
		if err := os.WriteFile(filepath.Join(destDir, outName), data, 0644); err != nil {
			return MirrorResult{}, fmt.Errorf("write %s: %w", outName, err)
		}
		written = append(written, outName)
	}

	sort.Strings(written)
	return MirrorResult{Source: src.Name, Files: written}, nil
}

func fetchRelease(apiURL string) (*releaseResponse, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	authHeader(req)

	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &release, nil
}

func downloadAsset(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	authHeader(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
