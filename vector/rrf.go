package vector

import "sort"

const defaultRRFK = 60

func RRF(rankings [][]Result,k int)[]Result{
	if k <= 0{
		k = defaultRRFK
	}

	// map doc id to fused score
	scores := make(map[string]float32)

	for _,ranking := range rankings{
		for i,res := range ranking{
			rank := float32(i+1)
			scores[res.ID] += 1.0/(float32(k) + rank)
		}
	}

	// convert map to slice
	results := make([]Result,0,len(scores))

	for id,score := range scores{
		results = append(results,Result{
			ID: id,
			Score: score,
		})
	}

	// sort by fused score
	sort.Slice(results,func(i, j int) bool {
		return results[i].Score>results[j].Score
	})

	return results
}