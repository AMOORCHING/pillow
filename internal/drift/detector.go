package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// StatusCallback is called when drift status changes.
type StatusCallback func(status string, reason string)

// TokenCallback is called to track drift check costs.
type TokenCallback func(input, output int)

// Detector checks if the agent is drifting from its goal.
type Detector struct {
	apiKey    string
	model     string
	interval  int           // check every N tool calls
	pauseMs   int           // or on pause > this duration
	cooldownS int           // suppress after narration

	mu              sync.Mutex
	sessionGoal     string
	recentToolCalls []agent.AgentEvent
	lastCheckAt     int
	eventCount      int
	lastEventTime   time.Time
	lastNarration   time.Time
	checking        bool // prevent stacking

	onStatus StatusCallback
	onTokens TokenCallback
}

// NewDetector creates a drift detector.
func NewDetector(apiKey string, interval, pauseMs, cooldownS int) *Detector {
	if interval == 0 {
		interval = 10
	}
	if pauseMs == 0 {
		pauseMs = 2000
	}
	if cooldownS == 0 {
		cooldownS = 30
	}
	return &Detector{
		apiKey:    apiKey,
		model:     "claude-haiku-4-5-20251001",
		interval:  interval,
		pauseMs:   pauseMs,
		cooldownS: cooldownS,
	}
}

// SetCallbacks configures the drift detector's callbacks.
func (d *Detector) SetCallbacks(onStatus StatusCallback, onTokens TokenCallback) {
	d.onStatus = onStatus
	d.onTokens = onTokens
}

// SetSessionGoal sets the current session goal.
func (d *Detector) SetSessionGoal(goal string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.sessionGoal = goal
	d.recentToolCalls = nil
	d.lastCheckAt = 0
	d.eventCount = 0
}

// OnEvent processes an agent event and triggers drift checks when appropriate.
func (d *Detector) OnEvent(ctx context.Context, evt agent.AgentEvent) {
	if evt.Type != "preToolUse" {
		return
	}

	d.mu.Lock()
	d.eventCount++
	d.recentToolCalls = append(d.recentToolCalls, evt)
	if len(d.recentToolCalls) > 10 {
		d.recentToolCalls = d.recentToolCalls[1:]
	}

	shouldCheck := false

	// Natural pause check
	if !d.lastEventTime.IsZero() && time.Since(d.lastEventTime) > time.Duration(d.pauseMs)*time.Millisecond {
		shouldCheck = true
	}

	// Interval check
	if d.eventCount-d.lastCheckAt >= d.interval {
		shouldCheck = true
	}

	// Cooldown check — suppress if we recently narrated
	if time.Since(d.lastNarration) < time.Duration(d.cooldownS)*time.Second {
		shouldCheck = false
	}

	// Don't stack checks
	if d.checking {
		shouldCheck = false
	}

	d.lastEventTime = time.Now()

	if !shouldCheck || d.sessionGoal == "" || d.apiKey == "" {
		d.mu.Unlock()
		return
	}

	d.checking = true
	d.lastCheckAt = d.eventCount
	goal := d.sessionGoal
	events := make([]agent.AgentEvent, len(d.recentToolCalls))
	copy(events, d.recentToolCalls)
	d.mu.Unlock()

	// Async drift check
	go d.check(ctx, goal, events)
}

const driftPrompt = `An AI coding agent is working on this goal: "%s"

Here are its last %d tool calls:
%s

Is the agent on track toward the stated goal?
Respond with exactly one line:
ON_TRACK
DRIFTING: <one sentence explaining why>
OFF_TRACK: <one sentence explaining why>`

func (d *Detector) check(ctx context.Context, goal string, events []agent.AgentEvent) {
	defer func() {
		d.mu.Lock()
		d.checking = false
		d.mu.Unlock()
	}()

	toolCalls := formatToolCalls(events)
	prompt := fmt.Sprintf(driftPrompt, goal, len(events), toolCalls)

	reqBody := map[string]any{
		"model":      d.model,
		"max_tokens": 100,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", d.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[pillow] drift check error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[pillow] drift check API error %d: %s", resp.StatusCode, string(respBody))
		return
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	if d.onTokens != nil {
		d.onTokens(result.Usage.InputTokens, result.Usage.OutputTokens)
	}

	if len(result.Content) == 0 {
		return
	}

	text := strings.TrimSpace(result.Content[0].Text)

	if strings.HasPrefix(text, "ON_TRACK") {
		if d.onStatus != nil {
			d.onStatus("on_track", "")
		}
		return
	}

	var status, reason string
	if strings.HasPrefix(text, "DRIFTING:") {
		status = "possibly_drifting"
		reason = strings.TrimSpace(strings.TrimPrefix(text, "DRIFTING:"))
	} else if strings.HasPrefix(text, "OFF_TRACK:") {
		status = "off_track"
		reason = strings.TrimSpace(strings.TrimPrefix(text, "OFF_TRACK:"))
	} else {
		return
	}

	d.mu.Lock()
	d.lastNarration = time.Now()
	d.mu.Unlock()

	if d.onStatus != nil {
		d.onStatus(status, reason)
	}
}

func formatToolCalls(events []agent.AgentEvent) string {
	var buf bytes.Buffer
	for _, evt := range events {
		fmt.Fprintf(&buf, "- %s", evt.Tool)
		if path, ok := evt.Input["file_path"].(string); ok {
			fmt.Fprintf(&buf, " on %s", path)
		}
		if cmd, ok := evt.Input["command"].(string); ok {
			if len(cmd) > 60 {
				cmd = cmd[:60] + "..."
			}
			fmt.Fprintf(&buf, " (%s)", cmd)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}
