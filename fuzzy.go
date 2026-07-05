package main

import "unicode"

// fuzzyResult holds a match score and the rune indexes of matched characters,
// used by the TUI to highlight where the pattern hit.
type fuzzyResult struct {
	score   int
	indexes []int
}

func isWordBoundary(prev rune) bool {
	switch prev {
	case '/', '-', '_', '.', ' ':
		return true
	}
	return false
}

// fuzzyScore matches pattern against s as a subsequence, greedy left-to-right.
// pattern must already be lowercased by the caller. Scoring favours matches at
// word boundaries (start of string, after / - _ . or space, camelCase humps)
// and consecutive runs, and penalises gaps; shorter candidates win ties.
func fuzzyScore(s string, pattern []rune) (fuzzyResult, bool) {
	if len(pattern) == 0 {
		return fuzzyResult{}, true
	}
	runes := []rune(s)
	lower := make([]rune, len(runes))
	for i, r := range runes {
		lower[i] = unicode.ToLower(r)
	}

	indexes := make([]int, 0, len(pattern))
	score := 0
	si := 0
	prev := -2
	for _, p := range pattern {
		found := -1
		for ; si < len(lower); si++ {
			if lower[si] == p {
				found = si
				si++
				break
			}
		}
		if found < 0 {
			return fuzzyResult{}, false
		}
		switch {
		case found == 0 || isWordBoundary(lower[found-1]):
			score += 8
		case unicode.IsUpper(runes[found]) && unicode.IsLower(runes[found-1]):
			score += 6
		}
		if found == prev+1 {
			score += 4
		} else if prev >= 0 {
			gap := found - prev - 1
			if gap > 3 {
				gap = 3
			}
			score -= gap
		}
		indexes = append(indexes, found)
		prev = found
	}
	score -= len(runes) / 8
	return fuzzyResult{score: score, indexes: indexes}, true
}
