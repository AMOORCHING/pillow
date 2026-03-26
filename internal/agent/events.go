package agent

import "time"

// EventType represents the type of agent event.
type EventType int

const (
	EventInit       EventType = iota // session initialization
	EventThinking                    // agent reasoning/planning
	EventText                        // agent text output
	EventToolUse                     // tool invocation (file edit, command, etc.)
	EventToolResult                  // tool execution result
	EventComplete                    // task finished
	EventError                       // error occurred
)

func (e EventType) String() string {
	switch e {
	case EventInit:
		return "init"
	case EventThinking:
		return "thinking"
	case EventText:
		return "text"
	case EventToolUse:
		return "tool_use"
	case EventToolResult:
		return "tool_result"
	case EventComplete:
		return "complete"
	case EventError:
		return "error"
	default:
		return "unknown"
	}
}

// AgentEvent is a parsed event from the agent's output stream.
type AgentEvent struct {
	Type      EventType
	Timestamp time.Time

	// Init fields
	Model     string
	SessionID string

	// Thinking / Text fields
	Text string

	// ToolUse fields
	ToolName  string
	ToolInput map[string]any

	// ToolResult fields
	Stdout  string
	Stderr  string
	IsError bool

	// Complete fields
	Result  string
	CostUSD float64
	Usage   Usage
}

// Usage holds token usage data from the agent.
type Usage struct {
	InputTokens  int
	OutputTokens int
}
