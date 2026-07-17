package main

import "testing"

func TestSettingCatalogCoversUserFacingKeys(t *testing.T) {
	required := []string{
		"autostart", "keep.running", "nickname", "name", "volume", "density",
		"hear.muted", "hear.spatial", "hear.fade", "hear.keyboard", "hear.mouse", "hear.self",
		"share.keyboard", "share.mouse", "spatial.dynamic", "spatial.shuffleMinutes",
		"solo.keyboardVolume", "solo.mouseVolume",
		"batch.ms", "audio.device", "api.url", "ws.url",
	}
	seen := map[string]bool{}
	for _, item := range configSettingCatalog {
		if item.Key == "" || item.Label == "" || item.Description == "" {
			t.Fatalf("incomplete setting metadata: %+v", item)
		}
		seen[item.Key] = true
	}
	for _, key := range required {
		if !seen[key] {
			t.Fatalf("setting %s is missing from cliks set --list", key)
		}
	}
}
