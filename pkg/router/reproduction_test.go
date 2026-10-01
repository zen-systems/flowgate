package router

import (
	"testing"
)

func TestContainsTrigger_Reproduction(t *testing.T) {
	// "plan" is the trigger.
	// "planning" contains "plan" but is disqualified by word boundary.
	// "the plan" contains "plan" as a distinct word.
	// The current buggy implementation finds "plan" inside "planning", checks boundaries, fails, and returns false.
	// It never checks the second "plan" at the end.
	prompt := "planning the plan"
	trigger := "plan"

	if !containsTrigger(prompt, trigger) {
		t.Fatalf("Expected prompt '%s' to contain trigger '%s', but it did not.", prompt, trigger)
	}
}
