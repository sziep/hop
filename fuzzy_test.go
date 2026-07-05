package main

import (
	"slices"
	"testing"
)

func score(t *testing.T, s, pattern string) int {
	t.Helper()
	r, ok := fuzzyScore(s, []rune(pattern))
	if !ok {
		t.Fatalf("expected %q to match %q", pattern, s)
	}
	return r.score
}

func TestFuzzyScoreSubsequence(t *testing.T) {
	if _, ok := fuzzyScore("hop", []rune("hp")); !ok {
		t.Error("hp should match hop")
	}
	if _, ok := fuzzyScore("hop", []rune("ph")); ok {
		t.Error("ph should not match hop (order matters)")
	}
	if _, ok := fuzzyScore("hop", []rune("hopp")); ok {
		t.Error("pattern longer than candidate should not match")
	}
	if _, ok := fuzzyScore("hop", nil); !ok {
		t.Error("empty pattern should match anything")
	}
}

func TestFuzzyScoreCaseInsensitive(t *testing.T) {
	if _, ok := fuzzyScore("MyProject", []rune("myproj")); !ok {
		t.Error("match should be case-insensitive")
	}
}

func TestFuzzyScoreBoundaryBeatsMidWord(t *testing.T) {
	if score(t, "work", "wo") <= score(t, "network", "wo") {
		t.Error("boundary match should outscore mid-word match")
	}
	if score(t, "zalando/hop", "hop") <= score(t, "shophorn", "hop") {
		t.Error("post-slash boundary should outscore scattered match")
	}
}

func TestFuzzyScoreIndexes(t *testing.T) {
	r, ok := fuzzyScore("hop", []rune("hp"))
	if !ok || !slices.Equal(r.indexes, []int{0, 2}) {
		t.Errorf("expected indexes [0 2], got %v", r.indexes)
	}
}

func TestTruncatePathHl(t *testing.T) {
	s, hl := truncatePathHl("abcdefgh", []int{0, 7}, 5)
	if s != "…efgh" {
		t.Errorf("expected …efgh, got %q", s)
	}
	// index 0 fell off the front; index 7 maps to position 4 (after the ellipsis)
	if !slices.Equal(hl, []int{4}) {
		t.Errorf("expected shifted highlight [4], got %v", hl)
	}
	s, hl = truncatePathHl("abc", []int{1}, 5)
	if s != "abc" || !slices.Equal(hl, []int{1}) {
		t.Errorf("short strings should pass through unchanged, got %q %v", s, hl)
	}
}

func TestFrecencyScore(t *testing.T) {
	now := int64(1_000_000_000)
	fresh := &bookmark{UseCount: 1, LastUsed: now - 60}
	stale := &bookmark{UseCount: 1, LastUsed: now - 30*86400}
	never := &bookmark{}
	if frecencyScore(fresh, now) <= frecencyScore(stale, now) {
		t.Error("recent use should outscore stale use")
	}
	if frecencyScore(never, now) != 0 {
		t.Error("unused bookmark should score zero")
	}
}

func TestStoreRenameAndInsert(t *testing.T) {
	s := &store{}
	s.add("a", "/a")
	s.add("b", "/b")
	if !s.rename("a", "z") || s.byLabel("z") == nil || s.byLabel("a") != nil {
		t.Error("rename should relabel in place")
	}
	s.insertAt(1, &bookmark{Label: "mid", Path: "/mid"})
	if s.bookmarks[1].Label != "mid" || len(s.bookmarks) != 3 {
		t.Errorf("insertAt should splice, got %v", s.bookmarks)
	}
}
