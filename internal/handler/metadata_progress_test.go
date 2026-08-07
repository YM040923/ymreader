package handler

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/service"
)

func TestMetadataProgressEventIncludesWorkCoverIdentity(t *testing.T) {
	target := metadataTarget{
		EntityType: "work",
		EntityID:   "work-1",
		Title:      "作品",
		Work: &service.Work{
			ID:                    "work-1",
			CoverURL:              "/api/opds/work-cover/work-1",
			RepresentativeComicID: "comic-1",
		},
	}
	event := metadataProgressEvent(target, 1, 2)
	if event["entityId"] != "work-1" || event["comicId"] != "comic-1" {
		t.Fatalf("progress identity = %#v, want Work entity and representative Comic", event)
	}
	if event["coverUrl"] != "/api/opds/work-cover/work-1" {
		t.Fatalf("progress coverUrl = %#v", event["coverUrl"])
	}
}
