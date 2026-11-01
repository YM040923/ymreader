package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

// ComicDeleteDirectoryCount reports how many source directories the selected
// comics occupy before their database rows disappear.
func ComicDeleteDirectoryCount(comicIDs []string) int {
	return comicDirectoryCount(comicIDs)
}

// ManualWorkDeleteSpansDirectories only treats a manual/merged Work as
// expanded when its own member comics span several source directories. A bulk
// selection of independent single-directory Works does not need confirmation.
func ManualWorkDeleteSpansDirectories(comicIDs []string) (bool, error) {
	memberships, err := store.GetWorkMemberships(nil, false)
	if err != nil {
		return false, err
	}
	groups := make(map[string][]string)
	for _, comicID := range comicIDs {
		comicID = strings.TrimSpace(comicID)
		if workID := memberships[comicID]; workID != "" {
			groups[workID] = append(groups[workID], comicID)
		}
	}
	for _, group := range groups {
		if comicDirectoryCount(group) > 1 {
			return true, nil
		}
	}
	return false, nil
}

func comicDirectoryCount(comicIDs []string) int {
	directories := make(map[string]struct{}, len(comicIDs))
	seen := make(map[string]struct{}, len(comicIDs))
	for _, comicID := range comicIDs {
		comicID = strings.TrimSpace(comicID)
		if comicID == "" {
			continue
		}
		if _, ok := seen[comicID]; ok {
			continue
		}
		seen[comicID] = struct{}{}
		comic, err := store.GetComicByID(comicID)
		if err != nil || comic == nil {
			continue
		}
		relative := normalizeWorkPath(firstNonEmpty(comic.RelativePath, comic.Filename))
		if relative == "" {
			continue
		}
		directory := ""
		if index := strings.LastIndex(relative, "/"); index > 0 {
			directory = relative[:index]
		}
		directories[directory] = struct{}{}
	}
	return len(directories)
}

type ComicFileTarget struct {
	ComicID  string
	Path     string
	RootPath string
}

// ResolveComicFileTargets resolves physical files for a set of comics. Missing
// files are reported as warnings so deleting their records still succeeds.
func ResolveComicFileTargets(comicIDs []string) ([]ComicFileTarget, []string) {
	seen := make(map[string]struct{}, len(comicIDs))
	targets := make([]ComicFileTarget, 0, len(comicIDs))
	warnings := make([]string, 0)
	for _, comicID := range comicIDs {
		comicID = strings.TrimSpace(comicID)
		if comicID == "" {
			continue
		}
		if _, ok := seen[comicID]; ok {
			continue
		}
		seen[comicID] = struct{}{}
		resolved, err := GlobalFileResolver.ResolveContentPath(comicID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", comicID, err))
			continue
		}
		if resolved.LibraryID == "" || strings.TrimSpace(resolved.AbsolutePath) == "" {
			warnings = append(warnings, fmt.Sprintf("%s: unresolved library path", comicID))
			continue
		}
		cleanPath := filepath.Clean(resolved.AbsolutePath)
		if resolved.RootPath == "" || !pathWithin(filepath.Clean(resolved.RootPath), cleanPath) ||
			cleanPath == filepath.Clean(resolved.RootPath) {
			warnings = append(warnings, fmt.Sprintf("%s: path is outside its library root", comicID))
			continue
		}
		targets = append(targets, ComicFileTarget{
			ComicID:  comicID,
			Path:     cleanPath,
			RootPath: filepath.Clean(resolved.RootPath),
		})
	}
	return targets, warnings
}

// DeleteComicFileTargets removes resolved paths, skipping children of folders
// that are already being removed. Failures are returned per path.
func DeleteComicFileTargets(targets []ComicFileTarget) (int, []string) {
	sort.SliceStable(targets, func(i, j int) bool {
		return len(targets[i].Path) < len(targets[j].Path)
	})
	deleted := make([]string, 0, len(targets))
	failures := make([]string, 0)
	for _, target := range targets {
		cleanPath := filepath.Clean(target.Path)
		if cleanPath == "" || cleanPath == "." || cleanPath == string(filepath.Separator) {
			failures = append(failures, fmt.Sprintf("%s: refusing to delete invalid path", target.ComicID))
			continue
		}
		if target.RootPath == "" || !pathWithin(target.RootPath, cleanPath) || cleanPath == target.RootPath {
			failures = append(failures, fmt.Sprintf("%s: refusing to delete outside library root", target.ComicID))
			continue
		}
		covered := false
		for _, parent := range deleted {
			if pathWithin(parent, cleanPath) {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		if err := os.RemoveAll(cleanPath); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", cleanPath, err))
			continue
		}
		deleted = append(deleted, cleanPath)
	}
	return len(deleted), failures
}
