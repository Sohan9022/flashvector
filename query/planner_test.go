package query

import "testing"

// BenchmarkQueryPlanner measures the overhead of the decision-making logic
func BenchmarkQueryPlanner(b *testing.B) {
	vec := make([]float32, 384) // Standard dimension for AI models

	b.Run("Intent-Detection-Long", func(b *testing.B) {
		text := "How do I fix the broken water pipe in the north ward near the station?"
		for i := 0; i < b.N; i++ {
			Analyze(text) // Measures string analysis speed
		}
	})

	b.Run("Full-Plan-Hybrid", func(b *testing.B) {
		req := SearchRequest{
			Text:   "Sanitation issues",
			Vector: vec,
			K:      5,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			Plan(req) // Measures the full routing logic
		}
	})
}