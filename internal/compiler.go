package internal

import (
	"bytes"

	"github.com/sagernet/sing-box/common/srs"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
)

func Compile(ruleSet option.PlainRuleSet) ([]byte, error) {
	var buf bytes.Buffer
	if err := srs.Write(&buf, ruleSet, C.RuleSetVersionCurrent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
