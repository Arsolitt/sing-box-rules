package internal

import (
	"testing"

	C "github.com/sagernet/sing-box/constant"
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
		"github.com": false,
		"github.io":  false,
		"ghcr.io":    false,
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

	domainSuffix := []string(ruleSet.Rules[0].DefaultOptions.DomainSuffix)
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

	ipCIDR := []string(ruleSet.Rules[0].DefaultOptions.IPCIDR)
	if len(ipCIDR) != 0 {
		t.Errorf("expected 0 ip_cidr entries, got %d", len(ipCIDR))
	}
}
