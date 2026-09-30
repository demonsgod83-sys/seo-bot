package orchestrator

import (
	"strings"
	"testing"
)

//==========================================//
//         STATE MACHINE TESTS              //
//==========================================//

// TestTransitionAllowed verifies every edge in the allowedTransitions table
// returns nil when called directly. If you add a new state or edge, add a
// case here — the table and the tests must stay in sync.
func TestTransitionAllowed(t *testing.T) {
	valid := []struct {
		from AuditStatus
		to   AuditStatus
	}{
		// The only active path right now
		{StatusQueued, StatusCrawling},
		// Future path — defined now so the machine is complete
		{StatusCrawling, StatusParsing},
		{StatusParsing, StatusAnalyzing},
		{StatusAnalyzing, StatusAdvising},
		{StatusAdvising, StatusReporting},
		{StatusReporting, StatusCompleted},
		{StatusReporting, StatusCompletedWithWarnings},
		// Any step can fail
		{StatusQueued, StatusFailed},
		{StatusCrawling, StatusFailed},
		{StatusParsing, StatusFailed},
		{StatusAnalyzing, StatusFailed},
		{StatusAdvising, StatusFailed},
		{StatusReporting, StatusFailed},
	}

	for _, tt := range valid {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			if err := Transition(tt.from, tt.to); err != nil {
				t.Errorf("expected transition %s → %s to be allowed, got error: %v", tt.from, tt.to, err)
			}
		})
	}
}

// TestTransitionIllegal verifies that skipped, reversed, and arbitrary
// transitions all return a descriptive error — never silently succeed.
func TestTransitionIllegal(t *testing.T) {
	illegal := []struct {
		from        AuditStatus
		to          AuditStatus
		errContains string
	}{
		// Skipping a step
		{StatusQueued, StatusParsing, "illegal transition"},
		{StatusQueued, StatusCompleted, "illegal transition"},
		{StatusCrawling, StatusCompleted, "illegal transition"},
		// Going backwards
		{StatusParsing, StatusCrawling, "illegal transition"},
		{StatusAnalyzing, StatusCrawling, "illegal transition"},
		// Arbitrary jump
		{StatusQueued, StatusReporting, "illegal transition"},
		// From unknown state
		{"ghost_state", StatusCrawling, "unknown source state"},
	}

	for _, tt := range illegal {
		t.Run(string(tt.from)+"→"+string(tt.to), func(t *testing.T) {
			err := Transition(tt.from, tt.to)
			if err == nil {
				t.Fatalf("expected error for illegal transition %s → %s, got nil", tt.from, tt.to)
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

// TestTransitionFromTerminal verifies terminal states have no outgoing edges.
// An audit that is completed or failed must never move again.
func TestTransitionFromTerminal(t *testing.T) {
	terminals := []AuditStatus{
		StatusCompleted,
		StatusCompletedWithWarnings,
		StatusFailed,
	}

	// Try to move a terminal state to every other state
	allStates := []AuditStatus{
		StatusQueued,
		StatusCrawling,
		StatusParsing,
		StatusAnalyzing,
		StatusAdvising,
		StatusReporting,
		StatusCompleted,
		StatusCompletedWithWarnings,
		StatusFailed,
	}

	for _, terminal := range terminals {
		for _, target := range allStates {
			t.Run(string(terminal)+"→"+string(target), func(t *testing.T) {
				err := Transition(terminal, target)
				if err == nil {
					t.Errorf("terminal state %s should not be able to transition to %s", terminal, target)
				}
				if !strings.Contains(err.Error(), "terminal") {
					t.Errorf("error should mention 'terminal', got: %v", err)
				}
			})
		}
	}
}

// TestTheOneTransitionThatMattersRightNow is the Step 2 bar:
// audit.validated arrives → transition queued → crawling must succeed.
func TestTheOneTransitionThatMattersRightNow(t *testing.T) {
	if err := Transition(StatusQueued, StatusCrawling); err != nil {
		t.Fatalf("queued → crawling MUST be allowed — this is the only active transition: %v", err)
	}
}

// TestIsTerminal verifies terminal state detection is correct.
func TestIsTerminal(t *testing.T) {
	terminal := []AuditStatus{StatusCompleted, StatusCompletedWithWarnings, StatusFailed}
	nonTerminal := []AuditStatus{StatusQueued, StatusCrawling, StatusParsing, StatusAnalyzing, StatusAdvising, StatusReporting}

	for _, s := range terminal {
		t.Run("terminal_"+string(s), func(t *testing.T) {
			if !s.IsTerminal() {
				t.Errorf("%s should be terminal", s)
			}
		})
	}
	for _, s := range nonTerminal {
		t.Run("non_terminal_"+string(s), func(t *testing.T) {
			if s.IsTerminal() {
				t.Errorf("%s should NOT be terminal", s)
			}
		})
	}
}

// TestAllowedTransitionsTableIsComplete verifies that every AuditStatus constant
// has an entry in the allowedTransitions table. If you add a new status constant
// and forget to add its transitions, this test fails loudly.
func TestAllowedTransitionsTableIsComplete(t *testing.T) {
	allStatuses := []AuditStatus{
		StatusQueued,
		StatusCrawling,
		StatusParsing,
		StatusAnalyzing,
		StatusAdvising,
		StatusReporting,
		StatusCompleted,
		StatusCompletedWithWarnings,
		StatusFailed,
	}

	for _, s := range allStatuses {
		t.Run(string(s)+"_in_table", func(t *testing.T) {
			if _, exists := allowedTransitions[s]; !exists {
				t.Errorf("status %q has no entry in allowedTransitions — add it or the machine is incomplete", s)
			}
		})
	}
}
