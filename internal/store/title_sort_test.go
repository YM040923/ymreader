package store

import (
	"sort"
	"testing"
)

func TestBuildTitleSortKeyNaturalNumbers(t *testing.T) {
	titles := []string{"第10卷", "第2卷", "第1卷", "第02卷"}
	sort.SliceStable(titles, func(i, j int) bool {
		return BuildTitleSortKey(titles[i]) < BuildTitleSortKey(titles[j])
	})

	want := []string{"第1卷", "第2卷", "第02卷", "第10卷"}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("natural title order mismatch: got %v, want %v", titles, want)
		}
	}
}

func TestBuildTitleSortKeyChinesePinyinOrder(t *testing.T) {
	titles := []string{"张三", "王五", "李四", "阿部"}
	sort.SliceStable(titles, func(i, j int) bool {
		return BuildTitleSortKey(titles[i]) < BuildTitleSortKey(titles[j])
	})

	want := []string{"阿部", "李四", "王五", "张三"}
	for i := range want {
		if titles[i] != want[i] {
			t.Fatalf("pinyin title order mismatch: got %v, want %v", titles, want)
		}
	}
}

func TestTitleSortKeySQLFunction(t *testing.T) {
	dbPath := testDBPath(t)
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	var key string
	if err := db.QueryRow(`SELECT title_sort_key(?)`, "第10卷").Scan(&key); err != nil {
		t.Fatalf("title_sort_key SQL function failed: %v", err)
	}
	if key == "" {
		t.Fatal("title_sort_key SQL function returned empty key")
	}
}

func TestGetAllComicsTitleSortUsesNaturalOrder(t *testing.T) {
	dbPath := testDBPath(t)
	if err := InitDB(dbPath); err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer CloseDB()

	now := "2024-01-01T00:00:00Z"
	fixtures := []struct {
		id    string
		title string
	}{
		{"comic-10", "第10卷"},
		{"comic-2", "第2卷"},
		{"comic-a", "阿部"},
	}
	for _, f := range fixtures {
		if _, err := db.Exec(`INSERT INTO "Comic" ("id", "filename", "title", "pageCount", "fileSize", "addedAt", "updatedAt") VALUES (?, ?, ?, 0, 1, ?, ?)`,
			f.id, f.id+".cbz", f.title, now, now); err != nil {
			t.Fatalf("insert fixture %s failed: %v", f.id, err)
		}
	}

	result, err := GetAllComics(ComicListOptions{SortBy: "title", SortOrder: "asc"})
	if err != nil {
		t.Fatalf("GetAllComics failed: %v", err)
	}
	got := []string{result.Comics[0].Title, result.Comics[1].Title, result.Comics[2].Title}
	want := []string{"阿部", "第2卷", "第10卷"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("title order mismatch: got %v, want %v", got, want)
		}
	}
	for _, c := range result.Comics {
		if c.TitleSortKey == "" {
			t.Fatalf("comic %s has empty titleSortKey", c.ID)
		}
	}
}
