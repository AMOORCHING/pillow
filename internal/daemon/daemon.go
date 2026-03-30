package daemon

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/AMOORCHING/pillow/internal/agent"
	"github.com/AMOORCHING/pillow/internal/classify"
	"github.com/AMOORCHING/pillow/internal/config"
	"github.com/AMOORCHING/pillow/internal/cost"
	"github.com/AMOORCHING/pillow/internal/drift"
	"github.com/AMOORCHING/pillow/internal/history"
	"github.com/AMOORCHING/pillow/internal/narration"
	"github.com/AMOORCHING/pillow/internal/tts"
)

// Daemon is the core pillow daemon state. It implements ipc.EventHandler.
type Daemon struct {
	cfg *config.Config

	// Components
	tts        tts.Provider
	summarizer Summarizer
	queue      *narration.Queue
	filter     *narration.Filter
	tracker    *cost.Tracker
	drift      *drift.Detector
	history    *history.Store

	// Session state
	mu             sync.RWMutex
	sessionID      string
	sessionGoal    string
	eventCount     int
	events         []agent.AgentEvent // pending events for summarizer
	currentSummary string

	// Slap state
	slapMu    sync.Mutex
	slapEvent *agent.SlapEvent

	// Negotiation state
	negotiation NegotiationState

	// Drift detection
	driftStatus string
}

// Summarizer compresses agent events into narration text.
type Summarizer interface {
	Summarize(ctx context.Context, events []agent.AgentEvent, currentSummary string) (string, error)
}

// NegotiationState tracks slap-to-negotiate dialogue.
type NegotiationState struct {
	Active       bool
	AllowedFile  string
	Outcome      string // pending | stop | finish_file | revert | redirect
	RedirectText string
}

// New creates a new daemon with the given components.
func New(cfg *config.Config, ttsProvider tts.Provider, summarizer Summarizer) *Daemon {
	d := &Daemon{
		cfg:        cfg,
		tts:        ttsProvider,
		summarizer: summarizer,
		queue:      narration.NewQueue(time.Duration(cfg.Narration.StaleThresholdMs) * time.Millisecond),
		filter:     narration.NewFilter(),
		tracker:    cost.NewTracker(),
		history:    history.NewStore(),
	}

	// Set up drift detector if API key is available
	if cfg.Narration.AnthropicAPIKey != "" {
		d.drift = drift.NewDetector(cfg.Narration.AnthropicAPIKey, cfg.Drift.CheckInterval, cfg.Drift.PauseMs, cfg.Drift.CooldownS)
		d.drift.SetCallbacks(
			func(status string, reason string) {
				d.mu.Lock()
				d.driftStatus = status
				d.mu.Unlock()

				if status == "possibly_drifting" || status == "off_track" {
					narrationText := fmt.Sprintf("Drift detected: %s", reason)
					d.queue.Push(narrationText, narration.PriorityHigh)
					go d.speakNext(context.Background())
				}
			},
			func(input, output int) {
				d.tracker.AddDriftTokens(input, output)
			},
		)
	}

	return d
}

// HandleEvent processes an agent event from the plugin.
func (d *Daemon) HandleEvent(ctx context.Context, evt agent.AgentEvent) agent.EventResponse {
	d.mu.Lock()
	d.eventCount++
	d.events = append(d.events, evt)
	count := d.eventCount
	d.mu.Unlock()

	// Classify irreversibility
	level, reason := classify.Classify(evt.Tool, evt.Input)

	if d.drift != nil {
		d.drift.OnEvent(context.Background(), evt)
	}

	// Check silence-first filter
	d.mu.RLock()
	driftStatus := d.driftStatus
	d.mu.RUnlock()

	resp := agent.EventResponse{
		Classify: level,
		Reason:   reason,
	}

	if d.filter.ShouldNarrate(evt, level, driftStatus) {
		var narrationText string
		switch level {
		case "block":
			narrationText = fmt.Sprintf("About to %s %s — %s", evt.Tool, extractPath(evt.Input), reason)
		case "warn":
			narrationText = fmt.Sprintf("Note: %s on %s — %s", evt.Tool, extractPath(evt.Input), reason)
		}
		if narrationText != "" {
			resp.Narration = narrationText
			d.queue.Push(narrationText, narration.PriorityHigh)
			go d.speakNext(context.Background())
		}
	}

	// Trigger rolling summary compression at interval
	if d.cfg.Narration.SummaryInterval > 0 && count%d.cfg.Narration.SummaryInterval == 0 {
		go d.compressSummary(context.Background())
	}

	return resp
}

// HandleSessionStart initializes a new session.
func (d *Daemon) HandleSessionStart(_ context.Context, req agent.SessionStartRequest) {
	d.mu.Lock()
	d.sessionID = req.SessionID
	d.sessionGoal = req.Goal
	d.eventCount = 0
	d.events = nil
	d.currentSummary = req.Goal
	d.tracker = cost.NewTracker()
	d.driftStatus = ""

	if d.drift != nil {
		d.drift.SetSessionGoal(req.Goal)
	}
	d.mu.Unlock()

	log.Printf("[pillow] session started: %s (goal: %s)", req.SessionID, req.Goal)

	if d.tts != nil {
		d.tracker.AddTTSChars(len("Pillow is listening."))
		go d.tts.Speak(context.Background(), "Pillow is listening.")
	}
}

// HandleSessionEnd finalizes the session and returns cost + summary.
func (d *Daemon) HandleSessionEnd(ctx context.Context, req agent.SessionEndRequest) agent.SessionEndResponse {
	d.compressSummary(ctx)

	d.mu.RLock()
	summary := d.currentSummary
	d.mu.RUnlock()

	costSummary := d.tracker.Summary()
	endText := fmt.Sprintf("Wrapping up. Session complete. %s", costSummary)

	if d.tts != nil {
		d.tracker.AddTTSChars(len(endText))
		go d.tts.Speak(context.Background(), endText)
	}

	log.Printf("[pillow] session ended: %s", req.SessionID)

	return agent.SessionEndResponse{
		Cost:    costSummary,
		Summary: summary,
	}
}

// HandleNarrate immediately narrates the given text.
func (d *Daemon) HandleNarrate(ctx context.Context, req agent.NarrateRequest) {
	if d.tts != nil {
		d.tts.Speak(ctx, req.Text)
	}
	d.tracker.AddTTSChars(len(req.Text))
}

// GetSummary returns the current rolling summary.
func (d *Daemon) GetSummary() agent.SummaryResponse {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return agent.SummaryResponse{
		Summary:    d.currentSummary,
		EventCount: d.eventCount,
	}
}

// GetStatus returns the daemon's current status.
func (d *Daemon) GetStatus() agent.StatusResponse {
	d.mu.RLock()
	defer d.mu.RUnlock()
	resp := agent.StatusResponse{
		ActiveSession: d.sessionID,
		Events:        d.eventCount,
		Cost:          d.tracker.StatusLine(),
	}
	if d.negotiation.Active {
		resp.Negotiation = &agent.NegotiationInfo{
			Active:      true,
			AllowedFile: d.negotiation.AllowedFile,
			Outcome:     d.negotiation.Outcome,
		}
	}
	return resp
}

// PollSlap returns a buffered slap event if fresh, otherwise nil.
func (d *Daemon) PollSlap() *agent.SlapEvent {
	d.slapMu.Lock()
	defer d.slapMu.Unlock()

	if d.slapEvent == nil {
		return nil
	}

	// Check staleness
	if time.Since(d.slapEvent.Timestamp) > time.Duration(d.cfg.Interrupt.StaleMs)*time.Millisecond {
		d.slapEvent = nil
		return nil
	}

	evt := d.slapEvent
	d.slapEvent = nil
	d.tracker.AddSlap()
	return evt
}

// BufferSlap stores a slap event (called by the sensord client goroutine).
func (d *Daemon) BufferSlap(evt agent.SlapEvent) {
	d.slapMu.Lock()
	defer d.slapMu.Unlock()
	d.slapEvent = &evt
}

// LogInterrupt records an interrupt event to history.
func (d *Daemon) LogInterrupt(evt history.InterruptEvent) {
	if err := d.history.Append(evt); err != nil {
		log.Printf("[pillow] failed to log interrupt: %v", err)
	}
}

// Tracker returns the cost tracker.
func (d *Daemon) Tracker() *cost.Tracker {
	return d.tracker
}

func (d *Daemon) compressSummary(ctx context.Context) {
	d.mu.Lock()
	pending := make([]agent.AgentEvent, len(d.events))
	copy(pending, d.events)
	currentSummary := d.currentSummary
	d.events = nil
	d.mu.Unlock()

	if len(pending) == 0 {
		return
	}

	newSummary, err := d.summarizer.Summarize(ctx, pending, currentSummary)
	if err != nil {
		log.Printf("[pillow] summarizer error: %v", err)
		return
	}

	d.mu.Lock()
	d.currentSummary = newSummary
	d.mu.Unlock()
}

func (d *Daemon) speakNext(ctx context.Context) {
	item, ok := d.queue.Pop()
	if !ok {
		return
	}
	if d.tts != nil {
		if err := d.tts.Speak(ctx, item.Text); err != nil {
			log.Printf("[pillow] TTS error: %v", err)
		}
		d.tracker.AddTTSChars(len(item.Text))
	}
}

func extractPath(input map[string]any) string {
	if p, ok := input["file_path"].(string); ok {
		return p
	}
	if c, ok := input["command"].(string); ok {
		if len(c) > 50 {
			return c[:50] + "..."
		}
		return c
	}
	return "unknown"
}
