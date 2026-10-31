package handler

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/service"
)

func TestBuildWorkDirectoryNodes(t *testing.T) {
	works := []service.Work{
		{ID: "root", RootPath: "", CoverComicID: "c-root"},
		{ID: "one-piece", RootPath: "One Piece", CoverComicID: "c-op"},
		{ID: "series-a", RootPath: "Author/Series A", CoverURL: "/api/opds/work-cover/a"},
		{ID: "series-b", RootPath: "Author/Series B/Vol", CoverComicID: "c-sb"},
		{ID: "other", RootPath: "Author2/X"},
		{ID: "loose", RootPath: "Loose", Units: []service.WorkUnit{{RelativePath: "Loose.cbz"}}},
	}

	root := buildWorkDirectoryNodes(works, "")
	if len(root) != 3 {
		t.Fatalf("root directories = %d, want 3: %#v", len(root), root)
	}
	byPath := map[string]workDirectoryNode{}
	for _, node := range root {
		byPath[node.Path] = node
	}
	if node := byPath["One Piece"]; node.WorkCount != 1 || node.HasChildren {
		t.Fatalf("One Piece node = %#v, want count 1 without children", node)
	}
	if node := byPath["Author"]; node.WorkCount != 2 || !node.HasChildren {
		t.Fatalf("Author node = %#v, want count 2 with children", node)
	}
	if node := byPath["Author2"]; node.WorkCount != 1 || !node.HasChildren {
		t.Fatalf("Author2 node = %#v, want one work under a nested folder", node)
	}
	if node := byPath["Author"]; node.CoverURL != "/api/opds/work-cover/a" {
		t.Fatalf("Author cover = %q, want the first work cover", node.CoverURL)
	}

	author := buildWorkDirectoryNodes(works, "Author")
	if len(author) != 2 || author[0].Name != "Series A" || author[1].Name != "Series B" {
		t.Fatalf("Author children = %#v", author)
	}
	if author[1].Path != "Author/Series B" || !author[1].HasChildren {
		t.Fatalf("Series B node = %#v, want nested children", author[1])
	}
	if author[1].CoverComicID != "c-sb" {
		t.Fatalf("Series B cover = %q, want c-sb", author[1].CoverComicID)
	}

	seriesB := buildWorkDirectoryNodes(works, "Author/Series B")
	if len(seriesB) != 1 || seriesB[0].Name != "Vol" || seriesB[0].WorkCount != 1 {
		t.Fatalf("Series B children = %#v", seriesB)
	}

	if nodes := buildWorkDirectoryNodes(works, "One Piece"); len(nodes) != 0 {
		t.Fatalf("leaf directory should have no children, got %#v", nodes)
	}
}

func TestWorkDirectoryFilterMatchesExactRoot(t *testing.T) {
	works := []service.Work{
		{RootPath: ""},
		{RootPath: "One Piece"},
		{RootPath: "Author/Series A"},
		{RootPath: "Loose", Units: []service.WorkUnit{{RelativePath: "Loose.cbz"}}},
	}
	matches := filterWorks(works, workFilters{directory: "", directorySet: true})
	if len(matches) != 2 || matches[0].RootPath != "" || matches[1].RootPath != "Loose" {
		t.Fatalf("root filter = %#v, want the root work and loose file", matches)
	}
	matches = filterWorks(works, workFilters{directory: "Author/Series A", directorySet: true})
	if len(matches) != 1 || matches[0].RootPath != "Author/Series A" {
		t.Fatalf("directory filter = %#v", matches)
	}
	matches = filterWorks(works, workFilters{directory: "Author", directorySet: true})
	if len(matches) != 0 {
		t.Fatalf("parent directory should not include nested works, got %#v", matches)
	}
	if all := filterWorks(works, workFilters{}); len(all) != 4 {
		t.Fatalf("unfiltered works = %d, want 4", len(all))
	}
}

func TestValidWorkDirectory(t *testing.T) {
	valid := []string{"", "One Piece", "Author/Series A", "A/B/C"}
	invalid := []string{"/abs", "a//b", "a/../b", "../a", "a\\b", "a/\x00b"}
	for _, value := range valid {
		if !validWorkDirectory(value) {
			t.Errorf("validWorkDirectory(%q) = false, want true", value)
		}
	}
	for _, value := range invalid {
		if validWorkDirectory(value) {
			t.Errorf("validWorkDirectory(%q) = true, want false", value)
		}
	}
}
