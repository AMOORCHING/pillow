package agent

import "context"

// Parser parses agent output into structured events.
type Parser interface {
	// Parse reads from the agent process and emits events on the channel.
	// It blocks until the agent exits or the context is cancelled.
	Parse(ctx context.Context, events chan<- AgentEvent) error
}
