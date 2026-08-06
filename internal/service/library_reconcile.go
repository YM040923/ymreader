package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/archive"
	"github.com/nowen-reader/nowen-reader/internal/config"
	"github.com/nowen-reader/nowen-reader/internal/model"
	"github.com/nowen-reader/nowen-reader/internal/store"
)

type OwnershipRecordPreview struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	LibraryID    string `json:"libraryId"`
	LibraryName  string `json:"libraryName"`
	RelativePath string `json:"relativePath"`
}

type OwnershipIssue struct {
	PhysicalPath       string                   `json:"physicalPath"`
	TargetLibraryID    string                   `json:"targetLibraryId"`
	TargetLibraryName  string                   `json:"targetLibraryName"`
	TargetRelativePath string                   `json:"targetRelativePath"`
	TargetID           string                   `json:"targetId"`
	Action             string                   `json:"action"`
	Records            []OwnershipRecordPreview `json:"records"`
	Resolvable         bool                     `json:"resolvable"`
	TargetType         string                   `json:"-"`
}

type LibraryOwnershipPreview struct {
	Issues        []OwnershipIssue      `json:"issues"`
	RootConflicts []LibraryRootConflict `json:"rootConflicts"`
	IssueCount    int                   `json:"issueCount"`
	DuplicateRows int                   `json:"duplicateRows"`
	CanReconcile  bool                  `json:"canReconcile"`
}

type LibraryOwnershipReconcileResult struct {
	IssueCount int `json:"issueCount"`
	Reconciled int `json:"reconciled"`
	MergedRows int `json:"mergedRows"`
	MovedRows  int `json:"movedRows"`
	Blocked    int `json:"blocked"`
}

type ownershipResolvedRecord struct {
	record store.ComicOwnershipRecord
	path   string
}

func PreviewLibraryOwnership(rootOwnerOverrides ...map[string]string) (*LibraryOwnershipPreview, error) {
	libraries, err := store.GetAllLibraries()
	if err != nil {
		return nil, err
	}
	records, err := store.GetComicOwnershipRecords()
	if err != nil {
		return nil, err
	}
	var rootOwners map[string]string
	if len(rootOwnerOverrides) > 0 {
		rootOwners = rootOwnerOverrides[0]
	}
	ownership := NewLibraryOwnershipWithRootOwners(libraries, rootOwners)
	libraryByID := make(map[string]model.Library, len(libraries))
	for _, lib := range libraries {
		libraryByID[lib.ID] = lib
	}

	groups := make(map[string][]ownershipResolvedRecord)
	for _, record := range records {
		lib, ok := libraryByID[record.LibraryID]
		if !ok {
			continue
		}
		path, ok := resolveOwnershipRecordPath(record, lib)
		if !ok {
			continue
		}
		groups[path] = append(groups[path], ownershipResolvedRecord{record: record, path: path})
	}

	rootConflicts := ownership.ExactRootConflicts()
	if rootConflicts == nil {
		rootConflicts = []LibraryRootConflict{}
	}
	preview := &LibraryOwnershipPreview{
		Issues:        []OwnershipIssue{},
		RootConflicts: rootConflicts,
	}
	for physicalPath, group := range groups {
		owner, relativePath, ok := ownership.RelativePathForOwner(physicalPath)
		if !ok {
			continue
		}
		if info, statErr := os.Stat(physicalPath); statErr == nil && info.IsDir() {
			relativePath = strings.TrimSuffix(relativePath, "/") + "/"
		}
		targetID := store.PathToID(owner.LibraryID, relativePath)
		needsChange := len(group) > 1
		for _, item := range group {
			if item.record.ID != targetID || item.record.LibraryID != owner.LibraryID || filepath.ToSlash(item.record.RelativePath) != relativePath {
				needsChange = true
				break
			}
		}
		if !needsChange {
			continue
		}

		issue := OwnershipIssue{
			PhysicalPath:       physicalPath,
			TargetLibraryID:    owner.LibraryID,
			TargetLibraryName:  owner.LibraryName,
			TargetRelativePath: relativePath,
			TargetID:           targetID,
			TargetType:         ownershipTargetType(owner.LibraryType, relativePath, group),
			Resolvable:         true,
			Action:             "move",
		}
		if len(group) > 1 {
			issue.Action = "merge"
			preview.DuplicateRows += len(group) - 1
		}
		for _, item := range group {
			lib := libraryByID[item.record.LibraryID]
			issue.Records = append(issue.Records, OwnershipRecordPreview{
				ID:           item.record.ID,
				Title:        item.record.Title,
				LibraryID:    item.record.LibraryID,
				LibraryName:  lib.Name,
				RelativePath: item.record.RelativePath,
			})
		}
		preview.Issues = append(preview.Issues, issue)
	}

	sort.Slice(preview.Issues, func(i, j int) bool {
		return preview.Issues[i].PhysicalPath < preview.Issues[j].PhysicalPath
	})
	preview.IssueCount = len(preview.Issues)
	preview.CanReconcile = unresolvedRootConflictCount(ownership, preview.RootConflicts) == 0 && preview.IssueCount > 0
	return preview, nil
}

func ReconcileLibraryOwnership(rootOwnerOverrides ...map[string]string) (*LibraryOwnershipReconcileResult, error) {
	syncMu.Lock()
	if syncInProgress {
		syncMu.Unlock()
		return nil, fmt.Errorf("a library scan is already running")
	}
	syncInProgress = true
	syncMu.Unlock()
	defer func() {
		syncMu.Lock()
		syncInProgress = false
		syncMu.Unlock()
	}()

	var rootOwners map[string]string
	if len(rootOwnerOverrides) > 0 {
		rootOwners = rootOwnerOverrides[0]
	}
	preview, err := PreviewLibraryOwnership(rootOwners)
	if err != nil {
		return nil, err
	}
	libraries, loadErr := store.GetAllLibraries()
	if loadErr != nil {
		return nil, loadErr
	}
	ownership := NewLibraryOwnershipWithRootOwners(libraries, rootOwners)
	unresolved := unresolvedRootConflictCount(ownership, preview.RootConflicts)
	result := &LibraryOwnershipReconcileResult{
		IssueCount: preview.IssueCount,
		Blocked:    unresolved,
	}
	if unresolved > 0 {
		return result, fmt.Errorf("an explicit owner is required for each exact root conflict")
	}

	for _, issue := range preview.Issues {
		if !issue.Resolvable || len(issue.Records) == 0 {
			result.Blocked++
			continue
		}
		keeper := chooseOwnershipKeeper(issue)
		var duplicates []string
		var cacheSources []string
		for _, record := range issue.Records {
			cacheSources = append(cacheSources, record.ID)
			if record.ID != keeper.ID {
				duplicates = append(duplicates, record.ID)
			}
		}
		if err := store.ReconcileComicOwnership(
			keeper.ID,
			duplicates,
			issue.TargetID,
			issue.TargetLibraryID,
			issue.TargetRelativePath,
			issue.TargetType,
		); err != nil {
			return result, fmt.Errorf("reconcile %s: %w", issue.PhysicalPath, err)
		}
		for _, sourceID := range cacheSources {
			archive.MigrateThumbnailCache(sourceID, issue.TargetID)
			migratePageCache(sourceID, issue.TargetID)
		}
		result.Reconciled++
		result.MergedRows += len(duplicates)
		if keeper.ID != issue.TargetID || keeper.LibraryID != issue.TargetLibraryID || filepath.ToSlash(keeper.RelativePath) != issue.TargetRelativePath {
			result.MovedRows++
		}
	}
	InvalidateAllCaches()
	return result, nil
}

func unresolvedRootConflictCount(ownership *LibraryOwnership, conflicts []LibraryRootConflict) int {
	paths := make(map[string]bool)
	for _, conflict := range conflicts {
		canonical := canonicalPath(conflict.Path)
		if !ownership.RootConflictIsResolved(canonical) {
			paths[canonical] = true
		}
	}
	return len(paths)
}

func migratePageCache(sourceID, targetID string) {
	if sourceID == "" || targetID == "" || sourceID == targetID || filepath.Base(sourceID) != sourceID || filepath.Base(targetID) != targetID {
		return
	}
	cacheRoot := config.GetPagesCacheDir()
	sourcePath := filepath.Join(cacheRoot, sourceID)
	targetPath := filepath.Join(cacheRoot, targetID)
	if _, err := os.Stat(sourcePath); err != nil {
		return
	}
	if _, err := os.Stat(targetPath); err == nil {
		_ = os.RemoveAll(sourcePath)
		return
	}
	_ = os.Rename(sourcePath, targetPath)
}

func resolveOwnershipRecordPath(record store.ComicOwnershipRecord, lib model.Library) (string, bool) {
	relativePath := filepath.Clean(filepath.FromSlash(record.RelativePath))
	if filepath.IsAbs(relativePath) || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", false
	}
	for _, rootPath := range libraryRootPaths(lib) {
		path := filepath.Join(rootPath, relativePath)
		if _, err := os.Stat(path); err == nil {
			return canonicalPath(path), true
		}
	}
	return "", false
}

func chooseOwnershipKeeper(issue OwnershipIssue) OwnershipRecordPreview {
	for _, record := range issue.Records {
		if record.ID == issue.TargetID {
			return record
		}
	}
	for _, record := range issue.Records {
		if record.LibraryID == issue.TargetLibraryID {
			return record
		}
	}
	return issue.Records[0]
}

func ownershipTargetType(libraryType, relativePath string, group []ownershipResolvedRecord) string {
	switch libraryType {
	case "novel":
		return "novel"
	case "comic":
		return "comic"
	}
	lower := strings.ToLower(relativePath)
	if strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".epub") ||
		strings.HasSuffix(lower, ".mobi") || strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") {
		return "novel"
	}
	if config.IsSupportedArchive(relativePath) {
		return "comic"
	}
	for _, item := range group {
		if item.record.ComicType == "novel" {
			return "novel"
		}
	}
	return "comic"
}
