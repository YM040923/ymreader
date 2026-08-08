package service

import (
	"path"
	"strings"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

// buildScanRuleWorks converts a physical target selection into the same Work
// model used by the public catalog. Scan rules must not invent a second
// grouping algorithm, otherwise AI, directory organization, and the reader
// disagree about what constitutes one manga.
func buildScanRuleWorks(ids []string) ([]Work, error) {
	items := make([]store.ComicListItem, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		item, err := store.GetComicByID(id)
		if err != nil {
			return nil, err
		}
		if item != nil {
			items = append(items, *item)
		}
	}
	if len(items) == 0 {
		return []Work{}, nil
	}
	return BuildWorksFromComicList(items, WorkBuildOptions{
		ProbeInternalArchive: true,
		ResolveComicPath: func(item store.ComicListItem) (string, bool) {
			resolved, err := GlobalFileResolver.ResolveContentPath(item.ID)
			if err != nil || resolved.AbsolutePath == "" {
				return "", false
			}
			return resolved.AbsolutePath, true
		},
	}), nil
}

func workRootForComic(works []Work) map[string]string {
	result := make(map[string]string)
	for _, work := range works {
		for _, unit := range work.Units {
			if strings.TrimSpace(unit.ComicID) != "" {
				result[unit.ComicID] = work.RootPath
			}
		}
	}
	return result
}

// buildWorkAwareDirectoryOrganizeRelPath keeps a standalone archive as a
// single file while placing multi-file works below the Work root. It does not
// add another redundant Work directory for a file whose stem already equals
// the Work name.
func buildWorkAwareDirectoryOrganizeRelPath(oldRel, workRoot, strategy string) string {
	oldRel = normalizeScanRelPath(oldRel)
	workRoot = normalizeScanRelPath(workRoot)
	if oldRel == "" || workRoot == "" {
		return buildDirectoryOrganizeRelPath(oldRel, strategy)
	}
	if strings.HasSuffix(oldRel, "/") {
		return strings.TrimSuffix(workRoot, "/") + "/"
	}

	leaf := path.Base(strings.TrimSuffix(oldRel, "/"))
	workStem := path.Base(strings.TrimSuffix(workRoot, "/"))
	fileStem := strings.TrimSuffix(leaf, path.Ext(leaf))
	if strings.EqualFold(fileStem, workStem) {
		return oldRel
	}
	return path.Join(workRoot, leaf)
}
