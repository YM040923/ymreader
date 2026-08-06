package service

import "github.com/nowen-reader/nowen-reader/internal/store"

func PersistAndApplyLogicalWorks(works []Work) error {
	if !store.LogicalWorkPersistenceAvailable() {
		return nil
	}
	seeds := make([]store.LogicalWorkSeed, 0, len(works))
	workIDs := make([]string, 0, len(works))
	for _, work := range works {
		seeds = append(seeds, store.LogicalWorkSeed{
			ID:               work.ID,
			LibraryID:        work.LibraryID,
			RootPath:         work.RootPath,
			ContentType:      "comic",
			Title:            work.Title,
			CoverComicID:     work.CoverComicID,
			CoverAspectRatio: work.CoverAspectRatio,
		})
		workIDs = append(workIDs, work.ID)
	}
	if err := store.UpsertDetectedLogicalWorks(seeds); err != nil {
		return err
	}
	for _, work := range works {
		if err := store.SeedLogicalWorkRelations(work.ID, work.Tags, work.Categories); err != nil {
			return err
		}
	}
	persisted, err := store.GetLogicalWorksByIDs(workIDs)
	if err != nil {
		return err
	}
	tags, err := store.GetLogicalWorkTagsByWorkIDs(workIDs)
	if err != nil {
		return err
	}
	categories, err := store.GetLogicalWorkCategoriesByWorkIDs(workIDs)
	if err != nil {
		return err
	}
	ApplyLogicalWorkMetadata(works, persisted, tags, categories)
	return nil
}
