package server

import (
	"encoding/binary" // <-- ADDED
	"encoding/json"
	"flashvector/query"
	"flashvector/storage"
	"flashvector/vector"
	"math" // <-- ADDED
	"net/http"
)

// API holds our database store so the web routes can access it
type API struct {
	store *storage.Store
}

func NewAPI(store *storage.Store) *API {
	return &API{store: store}
}

// --- JSON Payloads ---

type InsertRequest struct {
	ID       string            `json:"id"`
	Vector   []float32         `json:"vector"`
	Metadata map[string]string `json:"metadata"`
}

type SearchRequest struct {
	Vector []float32         `json:"vector"`
	K      int               `json:"k"`
	Filter map[string]string `json:"filter"`
}


// --- Route Handlers ---

// HandleInsert receives a vector via POST and saves it to the WAL & Index
func (api *API) HandleInsert(w http.ResponseWriter, r *http.Request) {
	var req InsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Convert float array to bytes for storage
	valBytes := floatsToBytes(req.Vector)

	// Save to FlashVector!
	if err := api.store.Set(req.ID, valBytes, req.Metadata); err != nil {
		http.Error(w, "Failed to save vector", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"id":     req.ID,
	})
}

// // HandleSearch receives a query vector and returns the closest matches
// func (api *API) HandleSearch(w http.ResponseWriter, r *http.Request) {
// 	var req SearchRequest
// 	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
// 		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
// 		return
// 	}

// 	// Default K to 5 if not provided
// 	if req.K == 0 {
// 		req.K = 5
// 	}

// 	// Search FlashVector!
// 	results := api.store.VectorSearch(req.Vector, req.K, req.Filter)

// 	// Return results as JSON
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(results)
// }

// HandleSearch now uses the Query Planner to decide the best search strategy
func (api *API) HandleSearch(w http.ResponseWriter, r *http.Request) {
	// 1. Decode using the new SearchRequest that supports 'text' and 'vector'
	var req query.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// 2. Default K to 5 if not provided
	if req.K == 0 {
		req.K = 5
	}

	// 3. Ask the Planner for the best strategy and adaptive weight
	plan := query.Plan(req)

	var results []vector.Result

	// 4. Execute based on the Planner's decision
	switch plan.Strategy {
	case query.StrategyVectorOnly:
		// Only run vector search if no text was provided
		results = api.store.VectorSearch(req.Vector, req.K, nil)

	case query.StrategyKeywordOnly:
		// Only run keyword search if no vector was provided
		results = api.store.KeywordSearch(req.Text, req.K)

	case query.StrategyHybrid:
		// Run both and fuse them using the adaptive weight from the Planner
		results = api.store.AdaptiveSearch(req.Text, req.Vector, req.K, plan.RRFConstant)
	}

	// 5. Return results as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Start boots up the web server
func (api *API) Start(port string) error {
	mux := http.NewServeMux()
	
	// Register our two endpoints
	mux.HandleFunc("/insert", api.HandleInsert)
	mux.HandleFunc("/search", api.HandleSearch)

	return http.ListenAndServe(":"+port, mux)
}

// floatsToBytes correctly converts []float32 to []byte (4 bytes per float)
func floatsToBytes(floats []float32) []byte {
	b := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(b[i*4:(i+1)*4], bits)
	}
	return b
}