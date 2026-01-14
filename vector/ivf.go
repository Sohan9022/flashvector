package vector

import "sort"

type IVFIndex struct{
	centroids [][]float32
	lists map[int][]QuantizedVector
	probes int
	dim int
}

func (ivf *IVFIndex) Add(id string,vec []float32){
	// validate dim
	if len(vec) != ivf.dim{
		panic("vector dimension mismatch")
	}

	bestCentroid := -1
	bestScore := float32(-2.0)

	for i,centroid := range ivf.centroids{
		score := CosineSimilarity(vec,centroid)

		if score > bestScore{
			bestScore = score
			bestCentroid = i
		}
	}
        qv := Quantize(vec)
	    qv.id = id
		ivf.lists[bestCentroid] = append(ivf.lists[bestCentroid],qv)

}
		
	



func (ivf *IVFIndex) Search(query []float32,k int)[]Result{
	// validate dim
	if len(query) != ivf.dim{
		panic("vec dim mismatch")
	}

	// centroid score
	type centroidScore struct{
		id int
		score float32
	}

	scores := make([]centroidScore,0,len(ivf.centroids))

	for i,centroid := range ivf.centroids{
		score := CosineSimilarity(query,centroid)

		scores = append(scores,centroidScore{
			id : i,
			score :score,
		})
	}

		// sort centroid on similarity
		sort.Slice(scores,func(i,j int)bool{
			return scores[i].score>scores[j].score
		})

		// probe top clusters

		results := make([]Result,0)

		probeCount := ivf.probes

		if probeCount > len(ivf.centroids){
			probeCount = len(ivf.centroids)
		}

		for i := 0;i<probeCount;i++{
			centroidId := scores[i].id
			vectors := ivf.lists[centroidId]

			for _,v := range vectors{

				vec := Dequantize(v)

				score := CosineSimilarity(vec,query)

				results = append(results,Result{
					ID : v.id,
					Score: score,
				})
				// score := CosineSimilarity(v.values,query)

				// results = append(results,Result{
				// 	ID : v.ID,
				// 	Score: score,
				// })
			}
		}

		sort.Slice(results,func(i,j int)bool{
			return results[i].Score>results[j].Score
		})

		// return top k results
		if len(results)>k{
			return results[:k]
		}	

		return results
}

// constructor for ivf index

func NewIVFIndex(centroids [][]float32,probes int) *IVFIndex{
	// validat
	if len(centroids)==0{
		panic("ivfindex requires atleast one centroid")
	}

	dim := len(centroids[0])

	if dim == 0{
		panic("centroid dimension cannot be zero")
	}

	// validate all centroids have same dim
	for _,c := range centroids{
		if len(c) != dim{
			panic("centroid dim mismatch")
		}
	}

	// initialise inverted list
	lists := make(map[int][]QuantizedVector)

	for i := range centroids{
		lists[i] = make([]QuantizedVector,0)
	}

	if probes <=0{
		probes = 1
	}

	if probes > len(centroids){
		probes  = len(centroids)
	}

	return &IVFIndex{
		centroids : centroids,
		lists : lists,
		probes : probes,
		dim : dim,
	}
}

func (ivf *IVFIndex) Remove(id string){
	for centroidId,vectors := range ivf.lists{
		newVectors := make([]QuantizedVector,0,len(vectors))

		for _,qv := range vectors{
			if qv.id != id{
				newVectors = append(newVectors,qv)
			}
		}
		ivf.lists[centroidId] = newVectors
	}
}

// RebuildFromData clears the index and repopulates it from the snapshot data
func (ivf *IVFIndex) RebuildFromData(data map[string][]byte) {
	// 1. Clear existing inverted lists
	for i := range ivf.lists {
		ivf.lists[i] = make([]QuantizedVector, 0)
	}

	// 2. Re-add all vectors
	for id, bytes := range data {
		// bytesToVector is visible here because it's in the same 'vector' package (in index.go)
		vec := bytesToVector(bytes)
		ivf.Add(id, vec)
	}
}
