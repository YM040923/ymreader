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

// ApplyPersistedLogicalWorks overlays already-persisted Work metadata without
// creating or updating any database rows. Read handlers must use this function
// so a GET request can never contend with scanners or metadata writers.
func ApplyPersistedLogicalWorks(works []Work) error {
	if !store.LogicalWorkPersistenceAvailable() || len(works) == 0 {
		return nil
	}
	workIDs := make([]string, 0, len(works))
	for _, work := range works {
		workIDs = append(workIDs, work.ID)
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

// PersistDetectedWorksForLibraries rebuilds the logical Work layout after a
// content scan. It only persists local detection results and never performs an
// online metadata scrape.
func PersistDetectedWorksForLibraries(libraryIDs []string) error {
	if !store.LogicalWorkPersistenceAvailable() {
		return nil
	}
	libraryIDs = uniqueNonEmptyStrings(libraryIDs)
	result, err := store.GetAllComics(store.ComicListOptions{
		ContentType:      "comic",
		SortBy:           "title",
		SortOrder:        "asc",
		Page:             0,
		PageSize:         0,
		LibraryIDs:       libraryIDs,
		FilterLibraryIDs: len(libraryIDs) > 0,
	})
	if err != nil {
		return err
	}
	works := BuildWorksFromComicList(result.Comics, WorkBuildOptions{ProbeInternalArchive: true})
	if err := PersistAndApplyLogicalWorks(works); err != nil {
		return err
	}
	visibleByLibrary := make(map[string][]string)
	for _, work := range works {
		visibleByLibrary[work.LibraryID] = append(visibleByLibrary[work.LibraryID], work.ID)
	}
	for _, libraryID := range libraryIDs {
		if err := store.MarkMissingLogicalWorks(libraryID, visibleByLibrary[libraryID]); err != nil {
			return err
		}
	}
	return nil
}
