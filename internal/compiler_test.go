package internal

import (
	"bytes"
	"testing"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
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
