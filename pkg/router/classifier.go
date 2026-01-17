package router

// Classifier provides LLM-based prompt classification.
// This is a phase 2 feature - stub for now.
type Classifier struct {
	// adapter to use for classification
	adapterName string
	model       string
}

// NewClassifier creates a new LLM-based classifier.
// Phase 2: Will use an LLM to classify prompts into task types.
func NewClassifier(adapterName, model string) *Classifier {
	return &Classifier{
		adapterName: adapterName,
		model:       model,
	}
}

// Classify uses an LLM to determine the task type for a prompt.
// Phase 2: Returns task type and confidence score.
func (c *Classifier) Classify(prompt string) (taskType string, confidence float64, err error) {
	// Stub implementation - returns default with low confidence
	return "default", 0.0, nil
}
