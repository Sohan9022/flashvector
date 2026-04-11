package query

// SearchStrategy defines the custom type for our routing logic
type SearchStrategy int

// Define the constants so the compiler knows what StrategyHybrid, etc., are.
const (
	StrategyVectorOnly SearchStrategy = iota
	StrategyKeywordOnly
	StrategyHybrid
)

// SearchRequest defines the structure of the incoming user query.
// This must match what you expect from your API.
type SearchRequest struct {
	Text   string    `json:"text"`
	Vector []float32 `json:"vector"`
	K      int       `json:"k"`
	Filter map[string]string `json:"filter"`
}

// SearchPlan is the final "order" sent to the storage engine
type SearchPlan struct {
	Strategy    SearchStrategy
	RRFConstant int
}

// Plan looks at the request and decides how to search
func Plan(req SearchRequest) SearchPlan {
	hasText := len(req.Text) > 0
	hasVector := len(req.Vector) > 0

	plan := SearchPlan{
		Strategy:    StrategyKeywordOnly,
		RRFConstant: 60, // Default balanced weight
	}

	if hasText && hasVector {
		plan.Strategy = StrategyHybrid
		// Use the Analyzer (from the other file) to set the weight
		intent := Analyze(req.Text) 
		switch intent {
		case IntentSemantic:
			plan.RRFConstant = 20 // Heavy Vector
		case IntentExactMatch:
			plan.RRFConstant = 100 // Heavy Keyword
		default:
			plan.RRFConstant = 60
		}
	} else if hasVector {
		plan.Strategy = StrategyVectorOnly
	}

	return plan
}