package vector

import (
	"math/rand"
	"time"
)

// RandomCentroids generates random float vectors seeded with current timestamp
func RandomCentroids(count int, dim int) [][]float32 {
	return DeterministicCentroids(count, dim, time.Now().UnixNano())
}

// DeterministicCentroids generates reproducible centroids using a fixed seed
func DeterministicCentroids(count int, dim int, seed int64) [][]float32 {
	if count <= 0 {
		panic("centroid must be greater than 0")
	}

	if dim <= 0 {
		panic("centroid dim must be greater than 0")
	}

	rng := rand.New(rand.NewSource(seed))
	centroids := make([][]float32, count)

	for i := 0; i < count; i++ {
		vec := make([]float32, dim)
		for j := 0; j < dim; j++ {
			vec[j] = rng.Float32()
		}
		centroids[i] = vec
	}

	return centroids
}