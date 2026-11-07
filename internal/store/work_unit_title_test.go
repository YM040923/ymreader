package store

import "testing"

func TestWorkUnitTitleRoundTrip(t *testing.T) {
	setupTestDB(t)

	if err := SaveWorkUnitTitles([]WorkUnitTitle{
		{UnitID: "u1", WorkID: "w1", Title: "永夜降临", Source: "tencent"},
		{UnitID: "u2", WorkID: "w1", Title: "转世的御扶摇", Source: "tencent"},
	}); err != nil {
		t.Fatal(err)
	}
	titles, err := GetAllWorkUnitTitles()
	if err != nil {
		t.Fatal(err)
	}
	if titles["u1"] != "永夜降临" || titles["u2"] != "转世的御扶摇" {
		t.Fatalf("titles=%#v", titles)
	}

	// Upsert replaces instead of duplicating.
	if err := SaveWorkUnitTitles([]WorkUnitTitle{
		{UnitID: "u1", WorkID: "w1", Title: "新的标题", Source: "tencent"},
	}); err != nil {
		t.Fatal(err)
	}
	titles, err = GetAllWorkUnitTitles()
	if err != nil {
		t.Fatal(err)
	}
	if titles["u1"] != "新的标题" {
		t.Fatalf("upsert failed: %#v", titles)
	}

	if err := DeleteWorkUnitTitlesByUnitIDs([]string{"u1", "u2"}); err != nil {
		t.Fatal(err)
	}
	titles, err = GetAllWorkUnitTitles()
	if err != nil {
		t.Fatal(err)
	}
	if len(titles) != 0 {
		t.Fatalf("delete failed: %#v", titles)
	}
}
