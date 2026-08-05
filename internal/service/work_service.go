package service

import (
	"fmt"

	"github.com/nowen-reader/nowen-reader/internal/store"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

// RebuildAllWorks refreshes logical Work/WorkUnit projections for all enabled
// comic libraries. Novel libraries are intentionally excluded because the work
// model currently represents comic inventory only.
func RebuildAllWorks() error {
	libraries, err := store.GetAllLibraries()
	if err != nil {
		return err
	}
	for _, library := range libraries {
		if !library.Enabled || library.Type == "novel" {
			continue
		}
		if err := rebuildWorksForLibrary(library.ID); err != nil {
			return fmt.Errorf("rebuild works for library %s: %w", library.ID, err)
		}
	}
	return nil
}

// RebuildWorksForLibrary refreshes logical Work/WorkUnit projections for one
// library. Novel libraries are skipped.
func RebuildWorksForLibrary(libraryID string) error {
	library, err := store.GetLibraryByID(libraryID)
	if err != nil {
		return err
	}
	if library == nil {
		return fmt.Errorf("library %s not found", libraryID)
	}
	if library.Type == "novel" {
		return nil
	}
	return rebuildWorksForLibrary(libraryID)
}

func rebuildWorksForLibrary(libraryID string) error {
	items, err := store.ListWorkSourceItems(libraryID)
	if err != nil {
		return err
	}
	sourceItems := make([]workmodel.SourceItem, 0, len(items))
	for _, item := range items {
		sourceItems = append(sourceItems, workmodel.SourceItem{
			ID:           item.ID,
			LibraryID:    item.LibraryID,
			RelativePath: item.RelativePath,
			Title:        item.Title,
			FileSize:     item.FileSize,
			PageCount:    item.PageCount,
		})
	}
	return store.ReplaceWorksForLibrary(libraryID, workmodel.DetectWorks(sourceItems))
}
