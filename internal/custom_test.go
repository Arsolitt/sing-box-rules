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
