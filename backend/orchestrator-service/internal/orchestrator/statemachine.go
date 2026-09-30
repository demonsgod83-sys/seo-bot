package orchestrator

import (
	"fmt"
)

//==========================================//
//              STATE MACHINE               //
//==========================================//

// AuditStatus is the canonical lifecycle state of an audit.
// It is defined here — in the Orchestrator — because the Orchestrator is the
// single source of truth for "where is this audit right now."
// Every other service reports back to the Orchestrator; nothing else is
// allowed to change status directly.
type AuditStatus string

const (
	StatusQueued               AuditStatus = "queued"
	StatusCrawling             AuditStatus = "crawling"
	StatusParsing              AuditStatus = "parsing"
	StatusAnalyzing            AuditStatus = "analyzing"
	StatusAdvising             AuditStatus = "advising"
	StatusReporting            AuditStatus = "reporting"
	StatusCompleted            AuditStatus = "completed"
	StatusCompletedWithWarnings AuditStatus = "completed_with_warnings"
	StatusFailed               AuditStatus = "failed"
)

// allowedTransitions is the explicit state machine table.
//
// Rule: if a transition is not in this table, it is illegal.
// An illegal transition is a loud error — not a silent no-op.
// This means every future service that finishes its work and reports back
// MUST produce a status that is in the allowed set for the current state.
//
// Current active path:  queued → crawling → parsing → analyzing → advising → reporting → completed
// Terminal states:      completed, completed_with_warnings, failed  (no outgoing edges)
//
// Any step can transition to failed — that is universal and encoded below.
var allowedTransitions = map[AuditStatus][]AuditStatus{
	StatusQueued:    {StatusCrawling, StatusFailed},
	StatusCrawling:  {StatusParsing, StatusFailed},
	StatusParsing:   {StatusAnalyzing, StatusFailed},
	StatusAnalyzing: {StatusAdvising, StatusFailed},
	StatusAdvising:  {StatusReporting, StatusFailed},
	StatusReporting: {StatusCompleted, StatusCompletedWithWarnings, StatusFailed},
	// Terminal states — no outgoing edges.
	StatusCompleted:             {},
	StatusCompletedWithWarnings: {},
	StatusFailed:                {},
}

// terminalStatuses are states where no further work will happen.
// An audit in a terminal state is done — success or failure.
var terminalStatuses = map[AuditStatus]bool{
	StatusCompleted:             true,
	StatusCompletedWithWarnings: true,
	StatusFailed:                true,
}

func (s AuditStatus) IsTerminal() bool {
	return terminalStatuses[s]
}

//==========================================//
//          TRANSITION FUNCTION             //
//==========================================//

// Transition validates that moving from → to is permitted by the state machine.
// Returns a descriptive error if the transition is illegal — callers should
// treat this as a programming error or a corrupted event, not a user error.
func Transition(from, to AuditStatus) error {
	allowed, exists := allowedTransitions[from]
	if !exists {
		// from is not a known state at all
		return fmt.Errorf(
			"unknown source state %q — cannot transition to %q",
			from, to,
		)
	}

	for _, s := range allowed {
		if s == to {
			return nil
		}
	}

	if from.IsTerminal() {
		return fmt.Errorf(
			"illegal transition: %q is a terminal state and cannot move to %q",
			from, to,
		)
	}

	return fmt.Errorf(
		"illegal transition: %q → %q is not permitted (allowed from %q: %v)",
		from, to, from, allowed,
	)
}
