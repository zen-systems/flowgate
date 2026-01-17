package curator

// Decider determines if enough information has been gathered to answer the query.
type Decider struct {
	confidenceThreshold float64 // Minimum overall confidence to proceed
	requiredCoverage    float64 // Fraction of required needs that must be met
}

// NewDecider creates a new Decider.
func NewDecider(confidenceThreshold, requiredCoverage float64) *Decider {
	return &Decider{
		confidenceThreshold: confidenceThreshold,
		requiredCoverage:    requiredCoverage,
	}
}

// DefaultDecider returns a decider with sensible defaults.
func DefaultDecider() *Decider {
	return &Decider{
		confidenceThreshold: 0.7,
		requiredCoverage:    0.8,
	}
}

// DecisionResult contains the decision and reasoning.
type DecisionResult struct {
	CanAnswer      bool     // Whether we can proceed to answer
	ClarifyingQs   []string // Questions to ask user if we can't answer
	Confidence     float64  // Overall confidence score
	CoverageScore  float64  // Fraction of needs satisfied
	MissingCritical []string // Critical needs that are missing
	Reason         string   // Explanation of the decision
}

// Decide determines if the curator has enough information to answer.
func (d *Decider) Decide(needs []InformationNeed, gathered []GatheredInfo, gaps []Gap) DecisionResult {
	result := DecisionResult{}

	// If no needs, we can answer (trivial query)
	if len(needs) == 0 {
		result.CanAnswer = true
		result.Confidence = 1.0
		result.CoverageScore = 1.0
		result.Reason = "No specific information needs identified"
		return result
	}

	// Calculate coverage
	requiredCount := 0
	requiredSatisfied := 0
	totalNeeds := len(needs)
	totalSatisfied := 0

	needsSatisfied := make(map[string]float64)
	for _, info := range gathered {
		if info.Confidence > needsSatisfied[info.NeedID] {
			needsSatisfied[info.NeedID] = info.Confidence
		}
	}

	for _, need := range needs {
		conf := needsSatisfied[need.ID]
		if conf >= d.confidenceThreshold {
			totalSatisfied++
			if need.Required {
				requiredSatisfied++
			}
		}
		if need.Required {
			requiredCount++
		}
	}

	// Calculate scores
	if totalNeeds > 0 {
		result.CoverageScore = float64(totalSatisfied) / float64(totalNeeds)
	}
	if requiredCount > 0 {
		requiredCoverage := float64(requiredSatisfied) / float64(requiredCount)
		result.Confidence = requiredCoverage
	} else {
		result.Confidence = result.CoverageScore
	}

	// Check for critical gaps
	for _, gap := range gaps {
		if gap.Critical {
			result.MissingCritical = append(result.MissingCritical, gap.Need)
			if gap.ClarifyingQ != "" {
				result.ClarifyingQs = append(result.ClarifyingQs, gap.ClarifyingQ)
			}
		}
	}

	// Make decision
	if len(result.MissingCritical) > 0 {
		result.CanAnswer = false
		result.Reason = "Critical information is missing"
	} else if result.Confidence >= d.confidenceThreshold && result.CoverageScore >= d.requiredCoverage {
		result.CanAnswer = true
		result.Reason = "Sufficient information gathered"
	} else if result.Confidence < d.confidenceThreshold {
		result.CanAnswer = false
		result.Reason = "Confidence too low"
		// Add questions for low-confidence needs
		for _, gap := range gaps {
			if gap.ClarifyingQ != "" && !contains(result.ClarifyingQs, gap.ClarifyingQ) {
				result.ClarifyingQs = append(result.ClarifyingQs, gap.ClarifyingQ)
			}
		}
	} else {
		result.CanAnswer = false
		result.Reason = "Not enough information needs satisfied"
	}

	return result
}

// DecideSimple provides a quick yes/no decision.
func (d *Decider) DecideSimple(needs []InformationNeed, gathered []GatheredInfo, gaps []Gap) (canAnswer bool, clarifyingQs []string) {
	result := d.Decide(needs, gathered, gaps)
	return result.CanAnswer, result.ClarifyingQs
}

// ForceAnswer allows proceeding even when confidence is low.
// Returns clarifying questions that should be shown to user along with the answer.
func (d *Decider) ForceAnswer(needs []InformationNeed, gathered []GatheredInfo, gaps []Gap) (warnings []string) {
	result := d.Decide(needs, gathered, gaps)

	if !result.CanAnswer {
		if len(result.MissingCritical) > 0 {
			warnings = append(warnings, "Warning: Critical information is missing - "+result.MissingCritical[0])
		}
		if result.Confidence < d.confidenceThreshold {
			warnings = append(warnings, "Warning: Low confidence in gathered information")
		}
	}

	return warnings
}

// AdjustThresholds updates the decision thresholds.
func (d *Decider) AdjustThresholds(confidence, coverage float64) {
	if confidence > 0 && confidence <= 1 {
		d.confidenceThreshold = confidence
	}
	if coverage > 0 && coverage <= 1 {
		d.requiredCoverage = coverage
	}
}

// Strict returns a strict decider that requires high confidence.
func Strict() *Decider {
	return &Decider{
		confidenceThreshold: 0.9,
		requiredCoverage:    0.95,
	}
}

// Lenient returns a lenient decider that accepts lower confidence.
func Lenient() *Decider {
	return &Decider{
		confidenceThreshold: 0.5,
		requiredCoverage:    0.6,
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
