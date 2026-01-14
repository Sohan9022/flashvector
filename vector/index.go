package vector

import "sort"

// interface for vector index
type VectorIndex interface{
	Add(id string,vec []float32)
	Remove(id string)
	Search(query []float32,k int) []Result
	RebuildFromData(data map[string][]byte)
}

type Vector struct{
	ID string
	values []float32
}

type Index struct{
	vectors []Vector
}

type Result struct{
	ID string
	Score float32
}

func NewIndex() *Index{
	return &Index{
		vectors : make([]Vector,0),
	}
}

// add vectors to index
func (idx *Index) Add(ID string,value []float32){
	idx.vectors = append(idx.vectors,Vector{
		ID:ID,
		values:value,
	})
}

func (idx *Index) Search(query []float32,k int)[]Result{
	results := make([]Result,0)

	for _,v := range idx.vectors{

		Score := CosineSimilarity(query,v.values)

		results = append(results,Result{
			ID : v.ID,
			Score : Score,
		})
	}

	// sort results by Score descending
	sort.Slice(results,func(i ,j int)bool{
		return results[i].Score>results[j].Score
	})

	if len(results)>k{
		return results[:k]
	}

	return results
}

func (idx *Index) Remove(ID string){
	newvector := make([]Vector,0)

	for _,v := range idx.vectors{
		if v.ID != ID{
			newvector = append(newvector, v)
		}
	}

	idx.vectors = newvector

}

func bytesToVector(b []byte) []float32{
	vec := make([]float32,len(b))

	for i := range b{
		vec[i] = float32(b[i])
	}

	return vec
}

func (idx *Index) RebuildFromData(data map[string][]byte){
	idx.vectors = nil

	for k , v := range data{
		vec := bytesToVector(v)
		idx.Add(k,vec)
	}
}