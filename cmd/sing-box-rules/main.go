package main

import (
	"crypto/rand"
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
	externalPath := flag.String("external", "config/external.json", "path to external sources config")
	workDir := flag.String("work-dir", "", "working directory for git operations")
	dummy := flag.Bool("dummy", false, "generate dummy .srs files instead of fetching from API")
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
		if *dummy {
			log.Printf("generating dummy data for %s...", cfg.Domain)
			dummyData := make([]byte, 64+cfg.IntervalDays*16)
			rand.Read(dummyData)
			outputPath := filepath.Join(*workDir, cfg.Name+".srs")
			if err := os.WriteFile(outputPath, dummyData, 0644); err != nil {
				log.Printf("error writing %s: %v, skipping.", cfg.Name, err)
				continue
			}
			updatedDomains = append(updatedDomains, cfg.Name)
			log.Printf("updated %s.srs", cfg.Name)
			continue
		}

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

	var externalResults []internal.MirrorResult
	if *dummy {
		log.Println("dummy mode: skipping external sources")
	} else {
		log.Println("mirroring external sources...")
		externalSources, err := internal.LoadExternalConfig(*externalPath)
		if err != nil {
			log.Printf("warning: external config error: %v", err)
		} else {
			for _, src := range externalSources {
				result, err := internal.MirrorExternal(src, *workDir)
				if err != nil {
					log.Printf("error mirroring %s: %v, skipping.", src.Name, err)
					continue
				}
				log.Printf("mirrored %s: %d files", src.Name, len(result.Files))
				externalResults = append(externalResults, result)
			}
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
	commitMsg := buildCommitMessage(updatedDomains, customResults, externalResults)
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

func buildCommitMessage(domains []string, customResults []internal.CustomResult, externalResults []internal.MirrorResult) string {
	var parts []string
	if len(domains) > 0 {
		parts = append(parts, fmt.Sprintf("%s (%d domains)", strings.Join(domains, ", "), len(domains)))
	}
	if len(customResults) > 0 {
		parts = append(parts, fmt.Sprintf("custom (%d rules)", len(customResults)))
	}
	externalCount := 0
	for _, r := range externalResults {
		externalCount += len(r.Files)
	}
	if externalCount > 0 {
		parts = append(parts, fmt.Sprintf("external (%d files)", externalCount))
	}
	if len(parts) == 0 {
		return "update: (no changes)"
	}
	return "update: " + strings.Join(parts, ", ")
}
