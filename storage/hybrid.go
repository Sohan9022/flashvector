package storage

import "flashvector/vector"

func (s *Store) HybridSearch(query string,queryVector []float32,k int)[]vector.Result{
	// run keyword search
	keywordResults := s.KeywordSearch(query,k)

	// run vector search
	vectorResults := s.VectorSearch(queryVector,k)

	// fuse ranking using rrf
	rankings := [][]vector.Result{
		keywordResults,
		vectorResults,
	}

	finalResults := vector.RRF(rankings,60)

	return finalResults
}

