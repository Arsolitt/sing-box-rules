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
