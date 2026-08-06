package service

import (
	"fmt"
	"sync"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

type ManualWorkScrapeResult struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

var libraryOperationMu sync.Mutex
var activeLibraryOperations = make(map[string]struct{})

func tryBeginLibraryOperation(libraryID string) bool {
	libraryOperationMu.Lock()
	defer libraryOperationMu.Unlock()
	if _, exists := activeLibraryOperations[libraryID]; exists {
		return false
	}
	activeLibraryOperations[libraryID] = struct{}{}
	return true
}

func endLibraryOperation(libraryID string) {
	libraryOperationMu.Lock()
	delete(activeLibraryOperations, libraryID)
	libraryOperationMu.Unlock()
}

// ScrapeLibraryWorks scrapes each eligible comic Work once. It is synchronous
// by design so the UI receives a complete, unambiguous result and no hidden
// background job can continue writing after the request appears finished.
func ScrapeLibraryWorks(libraryID string, force bool) (ManualWorkScrapeResult, error) {
	if libraryID == "" {
		return ManualWorkScrapeResult{}, fmt.Errorf("library id is required")
	}
	if !tryBeginLibraryOperation(libraryID) {
		return ManualWorkScrapeResult{}, fmt.Errorf("library scrape already running")
	}
	defer endLibraryOperation(libraryID)

	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "comic", LibraryIDs: []string{libraryID}, FilterLibraryIDs: true,
		SortBy: "title", SortOrder: "asc", Page: 0, PageSize: 0,
	})
	if err != nil {
		return ManualWorkScrapeResult{}, err
	}
	works := BuildWorksFromComicList(result.Comics, WorkBuildOptions{ProbeInternalArchive: true})
	if err := PersistAndApplyLogicalWorks(works); err != nil {
		return ManualWorkScrapeResult{}, err
	}

	out := ManualWorkScrapeResult{Total: len(works)}
	for _, work := range works {
		metadataPoor := work.MetadataSource == "" && work.Author == "" &&
			work.Description == "" && work.Genre == "" && work.Year == nil
		coverMissing := work.StoredCoverURL == "" && !work.CoverLocked
		if !force && !metadataPoor && !coverMissing {
			out.Skipped++
			continue
		}
		if err := ScrapeWorkMetadata(work, work.CoverLocked || work.StoredCoverURL != ""); err != nil {
			out.Failed++
			_ = store.UpdateLogicalWorkScrapeState(work.ID, "failed", err.Error(), nil)
			continue
		}
		out.Success++
		_ = store.UpdateLogicalWorkScrapeState(work.ID, "success", "", nil)
	}
	return out, nil
}
