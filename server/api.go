package server

import (
	"encoding/binary"
	"encoding/json"
	"flashvector/query"
	"flashvector/storage"
	"flashvector/vector"
	"math"
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
	Text     string            `json:"text"`
	Vector   []float32         `json:"vector"`
	Metadata map[string]string `json:"metadata"`
}

type SearchRequest struct {
	Text   string            `json:"text"`
	Vector []float32         `json:"vector"`
	K      int               `json:"k"`
	Filter map[string]string `json:"filter"`
}

// --- Route Handlers ---

// HandleInsert receives a vector and optional text via POST and saves it to the WAL & Index
func (api *API) HandleInsert(w http.ResponseWriter, r *http.Request) {
	var req InsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Convert float array to bytes for storage
	valBytes := floatsToBytes(req.Vector)

	// Attach text to metadata for inverted index indexing
	meta := req.Metadata
	if meta == nil {
		meta = make(map[string]string)
	}
	if req.Text != "" {
		meta["text"] = req.Text
	}

	// Save to FlashVector!
	if err := api.store.Set(req.ID, valBytes, meta); err != nil {
		http.Error(w, "Failed to save vector: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"id":     req.ID,
	})
}

// HandleSearch uses the Query Planner to decide the best search strategy (Vector, Keyword, or Hybrid)
func (api *API) HandleSearch(w http.ResponseWriter, r *http.Request) {
	var req query.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	if req.K == 0 {
		req.K = 5
	}

	plan := query.Plan(req)
	var results []vector.Result

	switch plan.Strategy {
	case query.StrategyVectorOnly:
		results = api.store.VectorSearch(req.Vector, req.K, req.Filter)

	case query.StrategyKeywordOnly:
		results = api.store.KeywordSearch(req.Text, req.K)

	case query.StrategyHybrid:
		results = api.store.AdaptiveSearch(req.Text, req.Vector, req.K, plan.RRFConstant)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Start boots up the web server
func (api *API) Start(port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/insert", api.HandleInsert)
	mux.HandleFunc("/search", api.HandleSearch)

	return http.ListenAndServe(":"+port, mux)
}

// floatsToBytes converts []float32 to little-endian []byte (4 bytes per float)
func floatsToBytes(floats []float32) []byte {
	b := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(b[i*4:(i+1)*4], bits)
	}
	return b
}