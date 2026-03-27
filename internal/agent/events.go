package agent

import "time"

// AgentEvent is the canonical event sent from plugin to daemon over IPC.
type AgentEvent struct {
	Type      string         `json:"type"`       // "preToolUse" | "postToolUse" | "sessionStart" | "sessionEnd"
	SessionID string         `json:"session_id"`
	Tool      string         `json:"tool"`        // "Write" | "Read" | "Bash" | "Edit" | "Glob" | "Grep" | etc.
	Input     map[string]any `json:"input"`       // tool-specific input (file_path, command, etc.)
	Output    string         `json:"output"`      // for postToolUse only
	Timestamp time.Time      `json:"timestamp"`
	Goal      string         `json:"goal"`        // for sessionStart only
}

// EventResponse is returned by the daemon to the plugin on POST /event.
type EventResponse struct {
	Classify  string `json:"classify"`  // "none" | "warn" | "block"
	Reason    string `json:"reason"`
	Narration string `json:"narration"` // text that will be narrated (if any)
}

// SlapEvent is returned by GET /slap when a slap is buffered.
type SlapEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Force     float64   `json:"force"`
}

// SessionStartRequest is sent on POST /session/start.
type SessionStartRequest struct {
	SessionID string `json:"session_id"`
	Goal      string `json:"goal"`
}

// SessionEndRequest is sent on POST /session/end.
type SessionEndRequest struct {
	SessionID string `json:"session_id"`
}

// SessionEndResponse is returned by POST /session/end.
type SessionEndResponse struct {
	Cost    string `json:"cost"`
	Summary string `json:"summary"`
}

// NarrateRequest is sent on POST /narrate.
type NarrateRequest struct {
	Text string `json:"text"`
}

// SummaryResponse is returned by GET /summary.
type SummaryResponse struct {
	Summary    string `json:"summary"`
	EventCount int    `json:"event_count"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	ActiveSession string          `json:"active_session"`
	Events        int             `json:"events"`
	Cost          string          `json:"cost"`
	Negotiation   *NegotiationInfo `json:"negotiation,omitempty"`
}

// NegotiationInfo describes an active slap negotiation.
type NegotiationInfo struct {
	Active      bool   `json:"active"`
	AllowedFile string `json:"allowed_file,omitempty"`
	Outcome     string `json:"outcome"`
}
