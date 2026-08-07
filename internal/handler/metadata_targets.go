package handler

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-reader/nowen-reader/internal/service"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

type metadataTarget struct {
	EntityType  string
	EntityID    string
	Title       string
	Author      string
	Filename    string
	ContentType string
	Work        *service.Work
	Comic       *store.ComicListItem
	LogicalWork *store.LogicalWork
}

type metadataTargetSelection struct {
	ID         string
	EntityType string
}

func discoverMetadataTargets(c *gin.Context, selections []metadataTargetSelection) ([]metadataTarget, error) {
	catalog, err := NewWorkHandler().loadWorkCatalog(c)
	if err != nil {
		return nil, err
	}
	workIDs := make([]string, 0, len(catalog.Works))
	for _, work := range catalog.Works {
		workIDs = append(workIDs, work.ID)
	}
	persisted, err := store.GetLogicalWorksByIDs(workIDs)
	if err != nil {
		return nil, err
	}

	targets := make([]metadataTarget, 0, len(catalog.Works))
	byWorkID := make(map[string]metadataTarget, len(catalog.Works))
	byComicID := make(map[string]metadataTarget)
	for index := range catalog.Works {
		work := catalog.Works[index]
		target := metadataTarget{
			EntityType: "work", EntityID: work.ID, Title: work.Title, Author: work.Author,
			Filename: work.RootPath, ContentType: "comic", Work: &work,
		}
		if logical, ok := persisted[work.ID]; ok {
			logicalCopy := logical
			target.LogicalWork = &logicalCopy
			if strings.TrimSpace(logicalCopy.Title) != "" {
				target.Title = logicalCopy.Title
			}
			target.Author = logicalCopy.Author
		}
		byWorkID[target.EntityID] = target
		for _, comicID := range service.PhysicalComicIDs(work) {
			byComicID[comicID] = target
		}
		targets = append(targets, target)
	}

	libraryIDs, filterLibraries, err := resolveWorkLibraryScope(c)
	if err != nil {
		return nil, err
	}
	novels, err := store.GetAllComics(store.ComicListOptions{
		ContentType: "novel", SortBy: "title", SortOrder: "asc", Page: 0, PageSize: 0,
		UserID: getUserID(c), LibraryIDs: libraryIDs, FilterLibraryIDs: filterLibraries,
	})
	if err != nil {
		return nil, err
	}
	byNovelID := make(map[string]metadataTarget, len(novels.Comics))
	for index := range novels.Comics {
		comic := novels.Comics[index]
		target := metadataTarget{
			EntityType: "comic", EntityID: comic.ID, Title: comic.Title, Author: comic.Author,
			Filename: comic.Filename, ContentType: "novel", Comic: &comic,
		}
		byNovelID[comic.ID] = target
		targets = append(targets, target)
	}

	if selections == nil {
		return targets, nil
	}
	selected := make([]metadataTarget, 0, len(selections))
	seen := make(map[string]struct{}, len(selections))
	for _, selection := range selections {
		id := strings.TrimSpace(selection.ID)
		if id == "" {
			continue
		}
		var target metadataTarget
		var ok bool
		switch strings.ToLower(strings.TrimSpace(selection.EntityType)) {
		case "work":
			target, ok = byWorkID[id]
		case "comic":
			if target, ok = byNovelID[id]; !ok {
				target, ok = byComicID[id]
			}
		default:
			if target, ok = byWorkID[id]; !ok {
				if target, ok = byNovelID[id]; !ok {
					target, ok = byComicID[id]
				}
			}
		}
		if !ok {
			continue
		}
		key := target.EntityType + ":" + target.EntityID
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, target)
	}
	return selected, nil
}

func metadataTargetMissing(target metadataTarget) bool {
	if target.EntityType == "work" {
		if target.LogicalWork == nil {
			return true
		}
		return strings.TrimSpace(target.LogicalWork.MetadataSource) == "" ||
			(strings.TrimSpace(target.LogicalWork.CoverURL) == "" &&
				strings.TrimSpace(target.LogicalWork.CoverComicID) == "")
	}
	if target.Comic == nil {
		return true
	}
	return strings.TrimSpace(target.Comic.MetadataSource) == ""
}

func metadataTargetSearchQuery(target metadataTarget) string {
	if target.EntityType == "work" {
		return strings.TrimSpace(strings.Join([]string{
			strings.TrimSpace(target.Title), strings.TrimSpace(target.Author),
		}, " "))
	}
	return service.BuildSearchQuery(target.Title, target.Filename)
}

func metadataProgressEvent(target metadataTarget, current, total int) gin.H {
	event := gin.H{
		"type": "progress", "current": current, "total": total,
		"entityType": target.EntityType, "entityId": target.EntityID, "title": target.Title,
	}
	if target.EntityType == "work" {
		comicID := ""
		if target.Work != nil {
			comicID = target.Work.RepresentativeComicID
			if comicID == "" {
				ids := service.PhysicalComicIDs(*target.Work)
				if len(ids) > 0 {
					comicID = ids[0]
				}
			}
			if target.Work.CoverURL != "" {
				event["coverUrl"] = target.Work.CoverURL
			}
		}
		event["comicId"] = comicID
		event["filename"] = target.Filename
	} else {
		event["comicId"] = target.EntityID
		event["filename"] = target.Filename
	}
	return event
}

func filterMissingMetadataTargets(targets []metadataTarget, mode string) []metadataTarget {
	if mode != "missing" {
		return targets
	}
	filtered := make([]metadataTarget, 0, len(targets))
	for _, target := range targets {
		if metadataTargetMissing(target) {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func requireMetadataTargetWork(target metadataTarget) (service.Work, error) {
	if target.Work == nil || target.EntityType != "work" {
		return service.Work{}, fmt.Errorf("metadata target %s is not a Work", target.EntityID)
	}
	return *target.Work, nil
}
