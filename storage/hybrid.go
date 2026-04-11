package storage

import (
	"flashvector/vector"
	"sync"
)
 
//explain
func (s *Store) AdaptiveSearch(text string, queryVector []float32, k int, rrfWeight int) []vector.Result {

	var keywordResults []vector.Result
	var vectorResults []vector.Result

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		keywordResults = s.KeywordSearch(text, k)
	}()

	go func() {
		defer wg.Done()
		vectorResults = s.VectorSearch(queryVector, k, nil)
	}()

	wg.Wait()

	rankings := make([][]vector.Result, 2)
	rankings[0] = keywordResults
	rankings[1] = vectorResults

	return vector.RRF(rankings, rrfWeight)
}

// AdaptiveSearch now receives the pre-calculated weight from the Planner
// func (s *Store) AdaptiveSearch(text string, queryVector []float32, k int, rrfWeight int) []vector.Result {
	
// 	// 1. Execute searches concurrently (you could wrap these in goroutines for even more speed later!)
// 	keywordResults := s.KeywordSearch(text, k)
// 	vectorResults := s.VectorSearch(queryVector, k, nil)

// 	// 2. Fuse using the smart weight decided by the Planner
// 	rankings := [][]vector.Result{
// 		keywordResults,
// 		vectorResults,
// 	}

// 	return vector.RRF(rankings, rrfWeight)
// }

//v1
// package storage

// import "flashvector/vector"

// func (s *Store) HybridSearch(query string,queryVector []float32,k int)[]vector.Result{
// 	// run keyword search
// 	keywordResults := s.KeywordSearch(query,k)

// 	// run vector search
// 	vectorResults := s.VectorSearch(queryVector,k,nil)

// 	// fuse ranking using rrf
// 	rankings := [][]vector.Result{
// 		keywordResults,
// 		vectorResults,
// 	}

// 	finalResults := vector.RRF(rankings,60)

// 	return finalResults
// }
