package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLegacyComicGroupScanRulesAreIgnored(t *testing.T) {
	var rules ScanRulesConfig
	if err := json.Unmarshal([]byte(`{
		"enabled": true,
		"aiInfer": {
			"enabled": true,
			"applyToComic": true,
			"applyToGroup": true
		},
		"organize": {
			"enabled": true,
			"autoGroupByDir": true,
			"inheritMeta": true
		}
	}`), &rules); err != nil {
		t.Fatal(err)
	}

	encoded, err := json.Marshal(rules)
	if err != nil {
		t.Fatal(err)
	}
	for _, removedField := range []string{"applyToGroup", "organize", "autoGroupByDir", "inheritMeta"} {
		if strings.Contains(string(encoded), removedField) {
			t.Errorf("legacy ComicGroup scan-rule field %q is still active: %s", removedField, encoded)
		}
	}
}
