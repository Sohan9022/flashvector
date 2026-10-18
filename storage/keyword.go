package storage

import (
	"sort"
	"strings"
	"unicode"

	"flashvector/vector"
)

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

// KeywordSearch uses the inverted index to look up matching documents by term frequency
func (s *Store) KeywordSearch(query string, k int) []vector.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	queryTokens := tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	scores := make(map[string]float32)

	// Direct Inverted Index lookup: token -> matching doc IDs
	for _, qt := range queryTokens {
		matchingDocs := s.inverted[qt]
		for _, docID := range matchingDocs {
			scores[docID]++
		}
	}

	// Fallback check: if inverted index didn't yield anything, check raw text in metadata or data
	if len(scores) == 0 {
		for id, meta := range s.meta {
			if docText, ok := meta["text"]; ok && docText != "" {
				docTokens := tokenize(docText)
				for _, qt := range queryTokens {
					for _, dt := range docTokens {
						if qt == dt {
							scores[id]++
						}
					}
				}
			}
		}
	}

	if len(scores) == 0 {
		return nil
	}

	results := make([]vector.Result, 0, len(scores))
	for id, score := range scores {
		results = append(results, vector.Result{
			ID:    id,
			Score: score,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > k {
		return results[:k]
	}

	return results
}
