package service

import "testing"

func TestProtectedCacheOwner(t *testing.T) {
	protected := []string{"work_6be280cc44f9532624a5", "group_13", "series_ser_123"}
	for _, owner := range protected {
		if !protectedCacheOwner(owner) {
			t.Fatalf("protectedCacheOwner(%q) = false, want true", owner)
		}
	}
	if protectedCacheOwner("a0e5bb105cc9") {
		t.Fatal("comic cache owner must not be protected")
	}
}

func TestThumbnailCacheOwnerID(t *testing.T) {
	cases := map[string]string{
		"abc123_300x400.webp":             "abc123",
		"legacy_id_with_underscore_300x400.webp": "legacy_id_with_underscore",
		"group_13_300x400.webp":           "group_13",
		"series_ser_123_300x400.webp":     "series_ser_123",
		"invalid.webp":                    "",
	}
	for name, want := range cases {
		if got := thumbnailCacheOwnerID(name); got != want {
			t.Fatalf("thumbnailCacheOwnerID(%q)=%q, want %q", name, got, want)
		}
	}
}
