package handler

import (
	"testing"

	"github.com/nowen-reader/nowen-reader/internal/store"
	"github.com/nowen-reader/nowen-reader/internal/workmodel"
)

func TestMetadataTranslationFieldsUseLogicalWorkMetadata(t *testing.T) {
	target := metadataTarget{
		EntityType: "work",
		EntityID:   "work-translate",
		Title:      "Work title",
		LogicalWork: &store.LogicalWork{
			ID:          "work-translate",
			Title:       "Logical title",
			Description: "Logical description",
			Genre:       "Action, Fantasy",
			Publisher:   "Logical publisher",
		},
	}

	fields := metadataTranslationFields(target)
	if fields["title"] != "Logical title" ||
		fields["description"] != "Logical description" ||
		fields["genre"] != "Action, Fantasy" ||
		fields["publisher"] != "Logical publisher" {
		t.Fatalf("Work translation fields = %#v", fields)
	}
}

func TestApplyMetadataTranslationUpdatesLogicalWorkInsteadOfRepresentativeComic(t *testing.T) {
	setupTestRouter(t)
	if err := store.RunMigrations(); err != nil {
		t.Fatal(err)
	}
	createOPDSHandlerLibrary(t, "translate-library", "comic", true)
	if err := store.UpsertDetectedLogicalWorks([]store.LogicalWorkSeed{{
		ID:          "ignored-seed-id",
		LibraryID:   "translate-library",
		RootPath:    "Translated Work",
		ContentType: "comic",
		Title:       "Original Work",
	}}); err != nil {
		t.Fatal(err)
	}
	workID := workmodel.StableWorkID("translate-library", "Translated Work")
	target := metadataTarget{
		EntityType: "work",
		EntityID:   workID,
		LogicalWork: &store.LogicalWork{
			ID: workID,
		},
	}

	if err := applyMetadataTranslation(target, map[string]string{
		"title":       "翻译后的作品",
		"description": "翻译后的简介",
		"genre":       "动作, 奇幻",
		"publisher":   "翻译后的出版社",
	}); err != nil {
		t.Fatal(err)
	}

	updated, err := store.GetLogicalWork(workID)
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil ||
		updated.Title != "翻译后的作品" ||
		updated.Description != "翻译后的简介" ||
		updated.Genre != "动作, 奇幻" ||
		updated.Publisher != "翻译后的出版社" {
		t.Fatalf("translated LogicalWork = %#v", updated)
	}
}
