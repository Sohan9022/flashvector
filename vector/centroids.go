package vector

import (
	"math/rand"
	"time"
)

func RandomCentroids(count int,dim int) [][]float32{
	if count <= 0{
		panic("centroid must be greater that 0")
	}

	if dim <= 0{
		panic("centroid dim must be greter than 0")
	}

	// seed random generator
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))


	centroids := make([][]float32,count)

	for i := 0;i<count;i++{
		vec := make([]float32,dim)

		for j := 0;j<dim;j++{
			vec[j] = rng.Float32()
		}

		centroids[i] = vec
	}

	return centroids
	
}