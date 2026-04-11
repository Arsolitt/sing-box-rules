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
