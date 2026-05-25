package engine

import (
	"regexp"
)

// matchesWord checks if intent appears as a whole word in text (case-insensitive).
func matchesWord(text, intent string) bool {
	if intent == "" {
		return false
	}
	quoted := regexp.QuoteMeta(intent)
	pattern := `(?i)(^|[^a-zA-Z0-9])` + quoted + `([^a-zA-Z0-9]|$)`
	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		return false
	}
	return matched
}

// findBestMatchSelector finds the observed element whose text best matches the
// intent text. Matching is case-insensitive word-boundary. Among matches, prefers
// the one with the closest text-length ratio (intent / element_text), preferring
// anchor tags and shorter text as tiebreakers. Returns (selector, true) on match.
func findBestMatchSelector(intentText string, elements []map[string]any) (string, bool) {
	if intentText == "" || len(elements) == 0 {
		return "", false
	}

	type candidate struct {
		selector string
		tag      string
		textLen  int
		score    float64
	}

	var best *candidate

	for _, elem := range elements {
		text, _ := elem["text"].(string)
		if text == "" {
			continue
		}
		if !matchesWord(text, intentText) {
			continue
		}

		sel, _ := elem["selector"].(string)
		if sel == "" {
			continue
		}

		tag, _ := elem["tag"].(string)
		score := float64(len(intentText)) / float64(len(text))

		if best == nil ||
			score > best.score ||
			(score == best.score && tag == "a" && best.tag != "a") ||
			(score == best.score && len(text) < best.textLen) {
			best = &candidate{selector: sel, tag: tag, textLen: len(text), score: score}
		}
	}

	if best != nil {
		return best.selector, true
	}
	return "", false
}
