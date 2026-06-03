package briefing

import "testing"

func TestLoadCachedBriefingEmailReadsFinalOutputCache(t *testing.T) {
	t.Parallel()

	cacheDir := t.TempDir()
	want := BriefingEmail{
		Subject:          "Cached final briefing",
		CriticalityScore: 7.5,
		PriorityLevel:    "Important",
	}
	if err := writeCacheJSON(cachedBriefingFilePath(cacheDir, finalBriefingOutputCacheFileName), want); err != nil {
		t.Fatalf("write cache JSON: %v", err)
	}

	got, err := LoadCachedBriefingEmail(cacheDir)
	if err != nil {
		t.Fatalf("LoadCachedBriefingEmail() error = %v", err)
	}
	if got.Subject != want.Subject || got.CriticalityScore != want.CriticalityScore || got.PriorityLevel != want.PriorityLevel {
		t.Fatalf("cached briefing = %#v, want %#v", got, want)
	}
}
