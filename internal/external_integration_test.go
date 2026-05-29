//go:build integration

// Integration tests hit live GitHub endpoints. Run with:
//
//	go test -tags integration -run Integration ./internal/ -v
//
// They are excluded from the default suite because they require network
// access and depend on third-party repositories staying available.
package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationMirrorBranchSingGeoIP(t *testing.T) {
	dir := t.TempDir()
	src := ExternalSource{
		Name:   "sing-geoip",
		Type:   "branch",
		Repo:   "SagerNet/sing-geoip",
		Branch: "rule-set",
		Prefix: "sagernet-",
	}
	result, err := MirrorExternal(src, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) < 100 {
		t.Fatalf("expected many geoip files, got %d", len(result.Files))
	}
	t.Logf("mirrored %d files from sing-geoip", len(result.Files))
	assertAllPrefixedValidSRS(t, dir, result.Files, "sagernet-geoip-")
}

func TestIntegrationMirrorBranchSingGeosite(t *testing.T) {
	dir := t.TempDir()
	src := ExternalSource{
		Name:   "sing-geosite",
		Type:   "branch",
		Repo:   "SagerNet/sing-geosite",
		Branch: "rule-set",
		Prefix: "sagernet-",
	}
	result, err := MirrorExternal(src, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) < 1000 {
		t.Fatalf("expected many geosite files, got %d", len(result.Files))
	}
	t.Logf("mirrored %d files from sing-geosite", len(result.Files))
	assertAllPrefixedValidSRS(t, dir, result.Files, "sagernet-geosite-")
}

func TestIntegrationMirrorReleaseItdoginfo(t *testing.T) {
	dir := t.TempDir()
	src := ExternalSource{
		Name:   "itdoginfo",
		Type:   "release",
		Repo:   "itdoginfo/allow-domains",
		Prefix: "itdog-",
	}
	result, err := MirrorExternal(src, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) < 10 {
		t.Fatalf("expected several itdoginfo .srs files, got %d", len(result.Files))
	}
	t.Logf("mirrored %d files from itdoginfo: %v", len(result.Files), result.Files)
	assertAllPrefixedValidSRS(t, dir, result.Files, "itdog-")
}

func assertAllPrefixedValidSRS(t *testing.T, dir string, files []string, prefix string) {
	t.Helper()
	for _, name := range files {
		if !strings.HasPrefix(name, prefix) {
			t.Errorf("file %s missing prefix %q", name, prefix)
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
			continue
		}
		if !IsValidSRS(data) {
			t.Errorf("file %s is not a valid SRS", name)
		}
	}
}
