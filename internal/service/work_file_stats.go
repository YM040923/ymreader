package service

import (
	"path"
	"sort"
	"strings"
	"time"

	"github.com/nowen-reader/nowen-reader/internal/store"
)

// BuildWorkFileStats projects physical file data through the Work model. The
// returned shape remains compatible with the existing statistics UI, while
// TotalFiles/ComicCount now mean logical works in Work scope.
func BuildWorkFileStats(works []Work) *store.FileStats {
	stats := &store.FileStats{
		FormatStats:      []store.FormatStatItem{},
		SizeDistribution: []store.SizeDistItem{},
		PageDistribution: []store.PageDistItem{},
		LanguageStats:    []store.LanguageStatItem{},
		AddedTimeline:    []store.AddedTimelineItem{},
		LargestFiles:     []store.LargestFileItem{},
		MostPages:        []store.LargestFileItem{},
		AuthorStats:      []store.AuthorStatItem{},
	}
	if len(works) == 0 {
		return stats
	}

	languageCounts := map[string]int{}
	authorCounts := map[string]int{}
	timelineCounts := map[string]int{}
	formatStats := map[string]store.FormatStatItem{}

	for _, work := range works {
		stats.TotalFiles++
		stats.ComicCount++
		stats.TotalSize += work.FileSize
		stats.TotalPages += work.PageCount
		if work.MetadataSource != "" || work.Author != "" ||
			work.Genre != "" || work.Description != "" {
			stats.WithMetadata++
		}
		language := strings.TrimSpace(work.Language)
		if language == "" {
			language = "未知"
		}
		languageCounts[language]++
		if author := strings.TrimSpace(work.Author); author != "" {
			authorCounts[author]++
		}
		if added, err := time.Parse(time.RFC3339, work.AddedAt); err == nil {
			timelineCounts[added.UTC().Format("2006-01")]++
		}

		format := workStatsFormat(work)
		formatItem := formatStats[format]
		formatItem.Format = format
		formatItem.Count++
		formatItem.TotalSize += work.FileSize
		formatStats[format] = formatItem

		item := store.LargestFileItem{
			ID:        work.ID,
			Title:     work.Title,
			Filename:  work.RootPath,
			FileSize:  work.FileSize,
			PageCount: work.PageCount,
			Type:      "work",
		}
		stats.LargestFiles = append(stats.LargestFiles, item)
		stats.MostPages = append(stats.MostPages, item)
	}
	stats.WithoutMetadata = stats.TotalFiles - stats.WithMetadata
	stats.AvgFileSize = stats.TotalSize / int64(stats.TotalFiles)
	stats.AvgPageCount = stats.TotalPages / stats.TotalFiles

	for _, item := range formatStats {
		stats.FormatStats = append(stats.FormatStats, item)
	}
	sort.Slice(stats.FormatStats, func(i, j int) bool {
		if stats.FormatStats[i].Count != stats.FormatStats[j].Count {
			return stats.FormatStats[i].Count > stats.FormatStats[j].Count
		}
		return stats.FormatStats[i].Format < stats.FormatStats[j].Format
	})

	stats.SizeDistribution = workSizeDistribution(works)
	stats.PageDistribution = workPageDistribution(works)
	stats.LanguageStats = languageStats(languageCounts)
	stats.AuthorStats = authorStats(authorCounts)
	stats.AddedTimeline = addedTimeline(timelineCounts)

	sort.Slice(stats.LargestFiles, func(i, j int) bool {
		return stats.LargestFiles[i].FileSize > stats.LargestFiles[j].FileSize
	})
	sort.Slice(stats.MostPages, func(i, j int) bool {
		return stats.MostPages[i].PageCount > stats.MostPages[j].PageCount
	})
	if len(stats.LargestFiles) > 10 {
		stats.LargestFiles = stats.LargestFiles[:10]
	}
	if len(stats.MostPages) > 10 {
		stats.MostPages = stats.MostPages[:10]
	}
	return stats
}

func workStatsFormat(work Work) string {
	formats := make(map[string]struct{})
	seenComic := make(map[string]struct{})
	for _, unit := range work.Units {
		if unit.ComicID != "" {
			if _, ok := seenComic[unit.ComicID]; ok {
				continue
			}
			seenComic[unit.ComicID] = struct{}{}
		}
		ext := strings.TrimPrefix(
			strings.ToUpper(path.Ext(unit.RelativePath)),
			".",
		)
		if ext == "" {
			ext = "FOLDER"
		}
		formats[ext] = struct{}{}
	}
	if len(formats) == 0 {
		ext := strings.TrimPrefix(strings.ToUpper(path.Ext(work.RootPath)), ".")
		if ext == "" {
			return "FOLDER"
		}
		return ext
	}
	if len(formats) > 1 {
		return "MIXED"
	}
	for format := range formats {
		return format
	}
	return "FOLDER"
}

func workSizeDistribution(works []Work) []store.SizeDistItem {
	result := []store.SizeDistItem{
		{Range: "< 10 MB"},
		{Range: "10 - 50 MB"},
		{Range: "50 - 200 MB"},
		{Range: "200 - 500 MB"},
		{Range: "> 500 MB"},
	}
	for _, work := range works {
		switch {
		case work.FileSize < 10*1024*1024:
			result[0].Count++
		case work.FileSize < 50*1024*1024:
			result[1].Count++
		case work.FileSize < 200*1024*1024:
			result[2].Count++
		case work.FileSize < 500*1024*1024:
			result[3].Count++
		default:
			result[4].Count++
		}
	}
	return result
}

func workPageDistribution(works []Work) []store.PageDistItem {
	result := []store.PageDistItem{
		{Range: "1 - 20 页"},
		{Range: "21 - 50 页"},
		{Range: "51 - 100 页"},
		{Range: "101 - 300 页"},
		{Range: "> 300 页"},
	}
	for _, work := range works {
		switch {
		case work.PageCount <= 0:
		case work.PageCount <= 20:
			result[0].Count++
		case work.PageCount <= 50:
			result[1].Count++
		case work.PageCount <= 100:
			result[2].Count++
		case work.PageCount <= 300:
			result[3].Count++
		default:
			result[4].Count++
		}
	}
	return result
}

func languageStats(counts map[string]int) []store.LanguageStatItem {
	result := make([]store.LanguageStatItem, 0, len(counts))
	for language, count := range counts {
		result = append(result, store.LanguageStatItem{
			Language: language,
			Count:    count,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	return result
}

func authorStats(counts map[string]int) []store.AuthorStatItem {
	result := make([]store.AuthorStatItem, 0, len(counts))
	for author, count := range counts {
		result = append(result, store.AuthorStatItem{Author: author, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	if len(result) > 10 {
		result = result[:10]
	}
	return result
}

func addedTimeline(counts map[string]int) []store.AddedTimelineItem {
	result := make([]store.AddedTimelineItem, 0, len(counts))
	for month, count := range counts {
		result = append(result, store.AddedTimelineItem{Month: month, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Month < result[j].Month })
	return result
}

// BuildWorkFolderTree uses Work.RootPath as the tree identity. Each Work
// appears exactly once even when it contains many physical CBZ files.
func BuildWorkFolderTree(works []Work) []*store.FolderTreeNode {
	root := &store.FolderTreeNode{
		Name:     "root",
		Path:     "",
		Children: []*store.FolderTreeNode{},
	}
	for _, work := range works {
		parts := splitWorkStatsPath(work.RootPath)
		if len(parts) == 0 {
			continue
		}
		directories := parts[:len(parts)-1]
		current := root
		pathSoFar := ""
		for _, directory := range directories {
			if pathSoFar == "" {
				pathSoFar = directory
			} else {
				pathSoFar += "/" + directory
			}
			current = findOrCreateWorkStatsNode(current, directory, pathSoFar)
		}
		current.Files = append(current.Files, store.FolderFileItem{
			ID:        work.ID,
			Title:     work.Title,
			Filename:  parts[len(parts)-1],
			FileSize:  work.FileSize,
			PageCount: work.PageCount,
			Type:      "work",
			LastRead:  work.ContinuePage,
		})
		accumulateWorkStats(root, directories, work)
	}
	if len(root.Children) == 0 {
		return []*store.FolderTreeNode{}
	}
	return root.Children
}

func splitWorkStatsPath(value string) []string {
	value = strings.ReplaceAll(strings.Trim(value, "/\\"), "\\", "/")
	if value == "" {
		return nil
	}
	raw := strings.Split(value, "/")
	result := make([]string, 0, len(raw))
	for _, part := range raw {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func findOrCreateWorkStatsNode(
	parent *store.FolderTreeNode,
	name, nodePath string,
) *store.FolderTreeNode {
	for _, child := range parent.Children {
		if child.Name == name {
			return child
		}
	}
	child := &store.FolderTreeNode{
		Name:     name,
		Path:     nodePath,
		Children: []*store.FolderTreeNode{},
	}
	parent.Children = append(parent.Children, child)
	return child
}

func accumulateWorkStats(
	root *store.FolderTreeNode,
	directories []string,
	work Work,
) {
	nodes := []*store.FolderTreeNode{root}
	current := root
	for _, directory := range directories {
		for _, child := range current.Children {
			if child.Name == directory {
				current = child
				nodes = append(nodes, child)
				break
			}
		}
	}
	for _, node := range nodes {
		node.FileCount++
		node.ComicCount++
		node.TotalSize += work.FileSize
		node.TotalPages += work.PageCount
		if strings.EqualFold(work.ReadingStatus, "finished") {
			node.ReadCount++
		}
	}
}
