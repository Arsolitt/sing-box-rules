package internal

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validSRS returns bytes with a valid SRS magic header.
func validSRS(payload string) []byte {
	return append([]byte{'S', 'R', 'S', 0x01}, []byte(payload)...)
}

// makeTarball builds a gzip-compressed tar stream from the given name->content map.
func makeTarball(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLoadExternalConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := `[
		{"name": "sing-geosite", "type": "branch", "repo": "SagerNet/sing-geosite", "branch": "rule-set", "prefix": "sagernet-"},
		{"name": "itdoginfo", "type": "release", "repo": "itdoginfo/allow-domains", "prefix": "itdog-"}
	]`
	path := filepath.Join(dir, "external.json")
	if err := os.WriteFile(path, []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}

	sources, err := LoadExternalConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Type != "branch" || sources[0].Branch != "rule-set" || sources[0].Prefix != "sagernet-" {
		t.Errorf("unexpected source[0]: %+v", sources[0])
	}
	if sources[1].Type != "release" || sources[1].Repo != "itdoginfo/allow-domains" || sources[1].Prefix != "itdog-" {
		t.Errorf("unexpected source[1]: %+v", sources[1])
	}
}

func TestLoadExternalConfigMissing(t *testing.T) {
	_, err := LoadExternalConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}

func TestLoadExternalConfigInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "external.json")
	os.WriteFile(path, []byte("not json {{{"), 0644)
	_, err := LoadExternalConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestIsValidSRS(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"valid magic", validSRS("payload"), true},
		{"magic only", []byte{'S', 'R', 'S', 0x01}, true},
		{"html error page", []byte("<!DOCTYPE html><html>404</html>"), false},
		{"wrong version byte", []byte{'S', 'R', 'S', 0x99, 'x'}, true}, // magic is SRS prefix; version byte may vary
		{"too short", []byte{'S', 'R'}, false},
		{"empty", []byte{}, false},
		{"plain text", []byte("not found"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidSRS(tc.data); got != tc.want {
				t.Errorf("IsValidSRS(%q) = %v, want %v", tc.data, got, tc.want)
			}
		})
	}
}

func TestExtractSRSFromTarball(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"sing-geoip-rule-set/geoip-ru.srs":     validSRS("ru-data"),
		"sing-geoip-rule-set/geoip-cn.srs":     validSRS("cn-data"),
		"sing-geoip-rule-set/README.md":        []byte("# readme"),         // not .srs, skip
		"sing-geoip-rule-set/broken.srs":       []byte("<html>404</html>"), // bad magic, skip
		"sing-geoip-rule-set/sub/geoip-de.srs": validSRS("de-data"),        // nested, basename only
	}
	tarball := makeTarball(t, files)

	written, err := extractSRSFromTarball(bytes.NewReader(tarball), dir, "sagernet-")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{
		"sagernet-geoip-ru.srs": true,
		"sagernet-geoip-cn.srs": true,
		"sagernet-geoip-de.srs": true,
	}
	if len(written) != len(want) {
		t.Fatalf("expected %d files, got %d: %v", len(want), len(written), written)
	}
	for _, name := range written {
		if !want[name] {
			t.Errorf("unexpected written file: %s", name)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("file %s not written: %v", name, err)
			continue
		}
		if !IsValidSRS(data) {
			t.Errorf("written file %s is not valid SRS", name)
		}
	}
	// broken.srs must not be written under any prefix
	if _, err := os.Stat(filepath.Join(dir, "sagernet-broken.srs")); !os.IsNotExist(err) {
		t.Error("broken.srs with bad magic should have been skipped")
	}
}

func TestExtractSRSFromTarballNoPrefix(t *testing.T) {
	dir := t.TempDir()
	tarball := makeTarball(t, map[string][]byte{
		"repo-branch/geosite-x.srs": validSRS("x"),
	})
	written, err := extractSRSFromTarball(bytes.NewReader(tarball), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 1 || written[0] != "geosite-x.srs" {
		t.Fatalf("expected [geosite-x.srs], got %v", written)
	}
}

func TestMirrorBranch(t *testing.T) {
	dir := t.TempDir()
	tarball := makeTarball(t, map[string][]byte{
		"sing-geosite-rule-set/geosite-youtube.srs": validSRS("yt"),
		"sing-geosite-rule-set/geosite-google.srs":  validSRS("goog"),
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/SagerNet/sing-geosite/tar.gz/refs/heads/rule-set"
		if r.URL.Path != wantPath {
			t.Errorf("unexpected path: %s, want %s", r.URL.Path, wantPath)
		}
		w.Header().Set("Content-Type", "application/x-gzip")
		w.Write(tarball)
	}))
	defer server.Close()

	src := ExternalSource{
		Name:   "sing-geosite",
		Type:   "branch",
		Repo:   "SagerNet/sing-geosite",
		Branch: "rule-set",
		Prefix: "sagernet-",
	}
	result, err := MirrorBranch(src, dir, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "sing-geosite" {
		t.Errorf("expected source name 'sing-geosite', got %q", result.Source)
	}
	if len(result.Files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(result.Files), result.Files)
	}
	for _, name := range result.Files {
		if !strings.HasPrefix(name, "sagernet-geosite-") {
			t.Errorf("file %s missing expected prefix", name)
		}
	}
}

func TestMirrorBranchServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	src := ExternalSource{Name: "x", Type: "branch", Repo: "a/b", Branch: "rule-set"}
	_, err := MirrorBranch(src, t.TempDir(), server.URL)
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestMirrorRelease(t *testing.T) {
	dir := t.TempDir()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/itdoginfo/allow-domains/releases/latest":
			assets := map[string]any{
				"assets": []map[string]string{
					{"name": "telegram.srs", "browser_download_url": server.URL + "/dl/telegram.srs"},
					{"name": "discord.srs", "browser_download_url": server.URL + "/dl/discord.srs"},
					{"name": "telegram_domain.mrs", "browser_download_url": server.URL + "/dl/telegram_domain.mrs"}, // not .srs
					{"name": "geosite.dat", "browser_download_url": server.URL + "/dl/geosite.dat"},                 // not .srs
				},
			}
			body, _ := json.Marshal(assets)
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		case strings.HasPrefix(r.URL.Path, "/dl/"):
			name := strings.TrimPrefix(r.URL.Path, "/dl/")
			w.Write(validSRS("data-for-" + name))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	src := ExternalSource{
		Name:   "itdoginfo",
		Type:   "release",
		Repo:   "itdoginfo/allow-domains",
		Prefix: "itdog-",
	}
	result, err := MirrorRelease(src, dir, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("expected 2 .srs files (mrs/dat filtered out), got %d: %v", len(result.Files), result.Files)
	}
	want := map[string]bool{"itdog-telegram.srs": true, "itdog-discord.srs": true}
	for _, name := range result.Files {
		if !want[name] {
			t.Errorf("unexpected file: %s", name)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !IsValidSRS(data) {
			t.Errorf("file %s not written or invalid: %v", name, err)
		}
	}
}

func TestMirrorReleaseSkipsInvalidAsset(t *testing.T) {
	dir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases/latest") {
			assets := map[string]any{
				"assets": []map[string]string{
					{"name": "good.srs", "browser_download_url": server.URL + "/dl/good.srs"},
					{"name": "bad.srs", "browser_download_url": server.URL + "/dl/bad.srs"},
				},
			}
			body, _ := json.Marshal(assets)
			w.Write(body)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/good.srs") {
			w.Write(validSRS("ok"))
			return
		}
		// bad.srs returns an HTML error page (invalid magic)
		w.Write([]byte("<html>Not Found</html>"))
	}))
	defer server.Close()

	src := ExternalSource{Name: "x", Type: "release", Repo: "a/b", Prefix: ""}
	result, err := MirrorRelease(src, dir, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0] != "good.srs" {
		t.Fatalf("expected only [good.srs], got %v", result.Files)
	}
}

func TestMirrorReleaseWithTag(t *testing.T) {
	dir := t.TempDir()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wantPath := "/repos/a/b/releases/tags/v1.0"
		if strings.HasSuffix(r.URL.Path, "/v1.0") {
			if r.URL.Path != wantPath {
				t.Errorf("unexpected tag path: %s, want %s", r.URL.Path, wantPath)
			}
			body, _ := json.Marshal(map[string]any{
				"assets": []map[string]string{
					{"name": "x.srs", "browser_download_url": server.URL + "/dl/x.srs"},
				},
			})
			w.Write(body)
			return
		}
		w.Write(validSRS("x"))
	}))
	defer server.Close()

	src := ExternalSource{Name: "x", Type: "release", Repo: "a/b", Tag: "v1.0"}
	result, err := MirrorRelease(src, dir, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected 1 file, got %v", result.Files)
	}
}

func TestMirrorExternalUnknownType(t *testing.T) {
	src := ExternalSource{Name: "x", Type: "ftp", Repo: "a/b"}
	_, err := MirrorExternal(src, t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown source type")
	}
	if !strings.Contains(err.Error(), "ftp") {
		t.Errorf("error should mention the unknown type, got: %v", err)
	}
}

// sanity: ensure the tarball helper round-trips through extract
func TestTarballHelperRoundTrip(t *testing.T) {
	dir := t.TempDir()
	content := validSRS("roundtrip")
	tarball := makeTarball(t, map[string][]byte{"r/file.srs": content})
	written, err := extractSRSFromTarball(bytes.NewReader(tarball), dir, "")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, written[0]))
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %q want %q", got, content)
	}
}
