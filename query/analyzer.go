package query

import (
	"strings"
	"unicode"
)

// QueryIntent describes what we think the user is trying to do
type QueryIntent int

const (
	IntentExactMatch QueryIntent = iota // Good for: IDs, single words, codes (Keyword Heavy)
	IntentSemantic                      // Good for: Questions, paragraphs (Vector Heavy)
	IntentBalanced                      // Good for: General short phrases (Equal mix)
)

// Analyze examines the text to determine the user's intent
func Analyze(text string) QueryIntent {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return IntentBalanced
	}

	// 1. Check if it's a question
	if strings.HasSuffix(text, "?") || hasQuestionWord(text) {
		return IntentSemantic
	}

	// 2. Check length (Long sentences are usually semantic)
	words := strings.Fields(text)
	if len(words) > 6 {
		return IntentSemantic
	}

	// 3. Check for IDs or Codes (e.g., "vec-12", "complaint_001")
	// If it has numbers or underscores/hyphens, they usually want an exact match
	if len(words) <= 2 && containsNumbersOrSymbols(text) {
		return IntentExactMatch
	}

	// 4. Very short searches (1-2 words) are usually exact keywords
	if len(words) <= 2 {
		return IntentExactMatch
	}

	return IntentBalanced
}

// --- Helper Functions ---

func hasQuestionWord(text string) bool {
	questions := []string{"what", "how", "why", "where", "who", "when", "can"}
	for _, q := range questions {
		if strings.HasPrefix(text, q+" ") {
			return true
		}
	}
	return false
}

func containsNumbersOrSymbols(text string) bool {
	for _, r := range text {
		if unicode.IsNumber(r) || r == '_' || r == '-' {
			return true
		}
	}
	return false
}