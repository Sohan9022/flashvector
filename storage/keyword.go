package storage


import(
	"strings"
	"unicode"
	"sort"

	"flashvector/vector"
)

func tokenize(text string) []string{
	return strings.FieldsFunc(strings.ToLower(text),func(r rune)bool{
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}


func (s *Store) KeywordSearch(query string ,k int)[]vector.Result{
	s.mu.RLock()
	defer s.mu.RUnlock()

	// tokenise query
	queryTokens := tokenize(query)

	if len(queryTokens) == 0{
		return nil
	}

	results := make([]vector.Result,0)

	// iterate over all stored doc
	for id,value := range s.data{
		text := string(value)
		docTokens := tokenize(text)

		// count matches
		score := 0

		for _,qt := range queryTokens{
			for _,dt := range docTokens{
				if qt == dt{
					score++
				}
			}
		}
		// keep only matching doc
		if score > 0{
			results = append(results, vector.Result{
				ID : id,
				Score : float32(score),
			})
		}

	}
	// sort by score
	sort.Slice(results,func(i ,j int)bool{
		return results[i].Score>results[j].Score
	})
	
	if len(results) > k{
		return results[:k]
	}

	return results
}

