package narration

import (
	"github.com/AMOORCHING/pillow/internal/agent"
)

// Filter implements the silence-first narration policy.
// If in doubt, return false — silence is the default.
type Filter struct{}

// NewFilter creates a new narration filter.
func NewFilter() *Filter {
	return &Filter{}
}

// ShouldNarrate returns true only if this event warrants breaking silence.
func (f *Filter) ShouldNarrate(evt agent.AgentEvent, classLevel string, driftStatus string) bool {
	// Irreversibility warnings/blocks always narrate
	if classLevel == "warn" || classLevel == "block" {
		return true
	}

	// Drift warnings narrate
	if driftStatus == "possibly_drifting" || driftStatus == "off_track" {
		return true
	}

	// Everything else: silence.
	return false
}
