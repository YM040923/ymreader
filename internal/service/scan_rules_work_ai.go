package service

import (
	"encoding/json"
	"fmt"
	"log"
	"path"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// runAIInferWorkAction performs one structured inference per Work and stores
// the result on LogicalWork. Physical Comic titles are deliberately not used
// as the metadata host anymore.
func runAIInferWorkAction(
	batchID string,
	works []Work,
	rule *config.AIInferRule,
	dryRun bool,
) (inferred, skipped, failed int) {
	cfg := LoadAIConfig()
	if !cfg.EnableCloudAI || strings.TrimSpace(cfg.CloudAPIKey) == "" {
		_ = store.InsertScanRuleOpLog(store.ScanRuleOpLog{
			BatchID: batchID,
			Action:  "ai_infer",
			Status:  "skipped",
			Message: "AI not configured",
		})
		return 0, len(works), 0
	}

	updateProgress(func(p *ScanRuleProgress) {
		p.Total = len(works)
		p.Current = 0
	})

	for _, work := range works {
		samples := workInferenceSamples(work)
		if len(samples) == 0 {
			skipped++
			updateProgress(func(p *ScanRuleProgress) {
				p.Skipped++
				p.Current++
			})
			continue
		}

		updateProgress(func(p *ScanRuleProgress) {
			p.CurrentDir = path.Base(strings.TrimSuffix(work.RootPath, "/"))
		})
		structured, err := AIInferTitleStructure(
			cfg,
			path.Base(strings.TrimSuffix(work.RootPath, "/")),
			samples,
			work.Title,
		)
		if err != nil {
			failed++
			updateProgress(func(p *ScanRuleProgress) {
				p.Failed++
				p.Current++
			})
			_ = store.InsertScanRuleOpLog(store.ScanRuleOpLog{
				BatchID: batchID,
				ComicID: work.RepresentativeComicID,
				Action:  "ai_infer",
				Status:  "failed",
				Message: fmt.Sprintf("work=%s: %v", work.ID, err),
			})
			continue
		}
		if structured == nil || strings.TrimSpace(structured.Title) == "" ||
			!meetsConfidence(structured.Confidence, rule.MinConfidence) {
			skipped++
			updateProgress(func(p *ScanRuleProgress) {
				p.Skipped++
				p.Current++
			})
			continue
		}

		applied, err := applyInferredToLogicalWork(
			work,
			structured,
			rule.OverwriteTitle,
			dryRun,
		)
		if err != nil {
			failed++
			updateProgress(func(p *ScanRuleProgress) {
				p.Failed++
				p.Current++
			})
			_ = store.InsertScanRuleOpLog(store.ScanRuleOpLog{
				BatchID: batchID,
				ComicID: work.RepresentativeComicID,
				Action:  "ai_infer",
				Status:  "failed",
				Message: fmt.Sprintf("work=%s: %v", work.ID, err),
			})
			continue
		}
		if !applied {
			skipped++
			updateProgress(func(p *ScanRuleProgress) {
				p.Skipped++
				p.Current++
			})
			continue
		}

		inferred++
		updateProgress(func(p *ScanRuleProgress) {
			p.Inferred++
			p.Current++
		})
		payload, _ := json.Marshal(structured)
		_ = store.InsertScanRuleOpLog(store.ScanRuleOpLog{
			BatchID:   batchID,
			ComicID:   work.RepresentativeComicID,
			Action:    "ai_infer",
			Status:    statusOf(dryRun),
			FromValue: work.Title,
			ToValue:   string(payload),
			Message:   "workId=" + work.ID,
		})
	}
	return
}

func workInferenceSamples(work Work) []string {
	samples := make([]string, 0, 8)
	seen := make(map[string]struct{})
	for _, unit := range work.Units {
		value := strings.TrimSpace(unit.DisplayLabel)
		if value == "" {
			value = strings.TrimSpace(unit.Title)
		}
		if value == "" && unit.RelativePath != "" {
			value = path.Base(strings.TrimSuffix(unit.RelativePath, "/"))
		}
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		samples = append(samples, value)
		if len(samples) == 8 {
			break
		}
	}
	if len(samples) == 0 && strings.TrimSpace(work.Title) != "" {
		samples = append(samples, work.Title)
	}
	return samples
}

func applyInferredToLogicalWork(
	work Work,
	structured *InferredTitleStructure,
	overwriteTitle bool,
	dryRun bool,
) (bool, error) {
	if structured == nil {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	logical, err := store.GetLogicalWork(work.ID)
	if err != nil {
		return false, err
	}
	if logical == nil {
		return false, fmt.Errorf("logical work not found: %s", work.ID)
	}
	if logical.MetadataLocked {
		return false, nil
	}
	if strings.TrimSpace(logical.MetadataSource) != "" &&
		!strings.EqualFold(logical.MetadataSource, "ai_scan_rules") {
		return false, nil
	}

	update := store.LogicalWorkMetadataUpdate{}
	if strings.TrimSpace(structured.Title) != "" &&
		(strings.TrimSpace(logical.Title) == "" || overwriteTitle) {
		title := strings.TrimSpace(structured.Title)
		update.Title = &title
	}
	if logical.Author == "" && structured.Author != "" {
		value := structured.Author
		update.Author = &value
	}
	if logical.Publisher == "" && structured.Publisher != "" {
		value := structured.Publisher
		update.Publisher = &value
	}
	if logical.Language == "" && structured.Language != "" {
		value := structured.Language
		update.Language = &value
	}
	if logical.Genre == "" && structured.Genre != "" {
		value := structured.Genre
		update.Genre = &value
	}
	if logical.Year == nil && structured.Year != nil {
		value := *structured.Year
		update.Year = &value
	}
	source := "ai_scan_rules"
	update.MetadataSource = &source
	if update.Title == nil && update.Author == nil && update.Publisher == nil &&
		update.Language == nil && update.Genre == nil && update.Year == nil {
		return false, nil
	}
	if err := store.UpdateLogicalWorkMetadata(work.ID, update); err != nil {
		return false, err
	}

	var tags []string
	if structured.ScanGroup != "" {
		tags = append(tags, "扫描组:"+structured.ScanGroup)
	}
	if structured.Version != "" {
		tags = append(tags, "版本:"+structured.Version)
	}
	if structured.Status != "" {
		tags = append(tags, "状态:"+structured.Status)
	}
	if len(tags) > 0 {
		if err := store.AddLogicalWorkAndComicTags([]string{work.ID}, nil, tags); err != nil {
			log.Printf("[scan-rules] logical work tags failed for %s: %v", work.ID, err)
		}
	}
	return true, nil
}
