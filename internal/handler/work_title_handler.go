package handler

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

// ============================================================
// 章节标题回填
//
// POST /api/works/:id/backfill-chapter-titles
// POST /api/works/backfill-chapter-titles       （整库）
// POST /api/works/:id/backfill-chapter-titles?clear=1  （清除该作品的回填）
//
// 只写入 WorkUnitTitle 表，不碰任何漫画文件；标题在目录接口返回时拼到
// 「第12话」这类纯编号名称后面。
// ============================================================

type chapterTitleBatchResponse struct {
	DryRun      bool                                 `json:"dryRun"`
	TotalFilled int                                  `json:"totalFilled"`
	Results     []service.ChapterTitleBackfillResult `json:"results"`
}

func (h *WorkHandler) BackfillChapterTitles(c *gin.Context) {
	workID := c.Param("id")
	works, err := h.loadWorks(c, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var target *service.Work
	for index := range works {
		if works[index].ID == workID {
			target = &works[index]
			break
		}
	}
	if target == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "work not found"})
		return
	}

	if c.Query("clear") == "1" {
		unitIDs := make([]string, 0, len(target.Units))
		for _, unit := range target.Units {
			unitIDs = append(unitIDs, unit.ID)
		}
		if err := store.DeleteWorkUnitTitlesByUnitIDs(unitIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resetWorkCatalogCache()
		c.JSON(http.StatusOK, gin.H{"cleared": len(unitIDs)})
		return
	}

	dryRun := c.Query("dryRun") == "1"
	result := service.CollectChapterTitleBackfill(*target, dryRun)
	if !dryRun && len(result.Overrides) > 0 {
		if err := saveChapterTitleOverrides(result.Overrides); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resetWorkCatalogCache()
	}
	c.JSON(http.StatusOK, result)
}

func (h *WorkHandler) BackfillAllChapterTitles(c *gin.Context) {
	works, err := h.loadWorks(c, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	dryRun := c.Query("dryRun") == "1"

	targets := make([]service.Work, 0)
	for _, work := range works {
		if service.WorkHasTitlelessChapters(work) {
			targets = append(targets, work)
		}
	}

	results := make([]service.ChapterTitleBackfillResult, len(targets))
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for index := range targets {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = service.CollectChapterTitleBackfill(targets[idx], dryRun)
		}(index)
	}
	wg.Wait()

	overrides := make([]service.ChapterTitleOverride, 0)
	for _, result := range results {
		overrides = append(overrides, result.Overrides...)
	}
	if !dryRun && len(overrides) > 0 {
		if err := saveChapterTitleOverrides(overrides); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resetWorkCatalogCache()
	}

	c.JSON(http.StatusOK, chapterTitleBatchResponse{
		DryRun:      dryRun,
		TotalFilled: len(overrides),
		Results:     results,
	})
}

func saveChapterTitleOverrides(overrides []service.ChapterTitleOverride) error {
	rows := make([]store.WorkUnitTitle, 0, len(overrides))
	for _, override := range overrides {
		rows = append(rows, store.WorkUnitTitle{
			UnitID: override.UnitID,
			WorkID: override.WorkID,
			Title:  override.Title,
			Source: override.Source,
		})
	}
	return store.SaveWorkUnitTitles(rows)
}
