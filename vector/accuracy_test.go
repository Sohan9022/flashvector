package vector

import (
	"fmt"
	"math/rand"
	"testing"
)

// Helper: Generates a random vector of given dimension
func randomVector(dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		// Generate random float between 0 and 1
		vec[i] = rand.Float32()
	}
	return vec
}

// Helper: Generates random centroids (simulating your missing utility)
func generateCentroids(dim int, count int) [][]float32 {
	centroids := make([][]float32, count)
	for i := range centroids {
		centroids[i] = randomVector(dim)
	}
	return centroids
}

func TestANNvsBruteForceRecall(t *testing.T) {
	// --- Settings ---
	dim := 64           // Vector dimensions
	numVectors := 5000  // Dataset size
	numCentroids := 50  // Number of IVF clusters
	probes := 15        // How many clusters to search (Higher = better recall, slower speed)
	k := 10             // Number of neighbors to find
	numQueries := 100   // Number of test queries to run

	// --- 1. Setup Indices ---
	// Brute Force Index
	bruteForce := NewIndex()

	// ANN (IVF) Index
	centroids := generateCentroids(dim, numCentroids)
	ann := NewIVFIndex(centroids, probes)

	// --- 2. Populate Data ---
	t.Logf("Generating and inserting %d vectors...", numVectors)
	for i := 0; i < numVectors; i++ {
		id := fmt.Sprintf("vec-%d", i)
		vec := randomVector(dim)
		
		bruteForce.Add(id, vec)
		ann.Add(id, vec)
	}

	// --- 3. Measure Recall ---
	totalRecall := 0.0

	for i := 0; i < numQueries; i++ {
		query := randomVector(dim)

		// Get "Ground Truth" from Brute Force
		truthResults := bruteForce.Search(query, k)

		// Get "Approximate" results from IVF
		annResults := ann.Search(query, k)

		// Calculate Intersection
		// (Count how many IDs from ANN appear in the Truth list)
		truthMap := make(map[string]bool)
		for _, r := range truthResults {
			truthMap[r.ID] = true
		}

		matches := 0
		for _, r := range annResults {
			if truthMap[r.ID] {
				matches++
			}
		}

		// Calculate Recall for this query
		if len(truthResults) > 0 {
			queryRecall := float64(matches) / float64(len(truthResults))
			totalRecall += queryRecall
		}
	}

	avgRecall := totalRecall / float64(numQueries)
	t.Logf("Configuration: %d Vectors, %d Centroids, %d Probes", numVectors, numCentroids, probes)
	t.Logf("Average Recall: %.2f%%", avgRecall*100)

	// --- 4. Validation ---
	// We expect decent recall (>70%) with reasonable probe counts.
	// If it's too low, the centroids might be poorly distributed or probes too low.
	if avgRecall < 0.7 {
		t.Errorf("Recall is too low (%.2f%%). Try increasing probe count.", avgRecall*100)
	}
}

// --- Benchmark: Compare Speed ---

func BenchmarkSearchBruteForce(b *testing.B) {
	idx := NewIndex()
	dim := 64
	for i := 0; i < 10000; i++ {
		idx.Add(fmt.Sprintf("%d", i), randomVector(dim))
	}
	query := randomVector(dim)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}

func BenchmarkSearchIVF(b *testing.B) {
	dim := 64
	centroids := generateCentroids(dim, 50)
	idx := NewIVFIndex(centroids, 5) // 5 probes
	
	for i := 0; i < 10000; i++ {
		idx.Add(fmt.Sprintf("%d", i), randomVector(dim))
	}
	query := randomVector(dim)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}