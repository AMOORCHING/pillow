package cost

import (
	"fmt"
	"sync"
	"time"
)

// Default rates (as of 2025)
const (
	CartesiaRatePerChar    = 0.000006
	HaikuInputRatePerTok   = 0.0000008
	HaikuOutputRatePerTok  = 0.000004
)

// Tracker estimates API costs in real-time.
type Tracker struct {
	mu sync.Mutex

	ttsChars      int
	ttsRate       float64
	llmInputToks  int
	llmOutputToks int
	llmInputRate  float64
	llmOutputRate float64
	agentCostUSD  float64

	sessionStart time.Time
	slapCount    int
	narrationCount int
}

// NewTracker creates a cost tracker with default rates.
func NewTracker() *Tracker {
	return &Tracker{
		ttsRate:      CartesiaRatePerChar,
		llmInputRate: HaikuInputRatePerTok,
		llmOutputRate: HaikuOutputRatePerTok,
		sessionStart: time.Now(),
	}
}

// AddTTSChars records characters sent to TTS.
func (t *Tracker) AddTTSChars(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ttsChars += n
	t.narrationCount++
}

// AddLLMTokens records tokens used by the summarizer.
func (t *Tracker) AddLLMTokens(input, output int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.llmInputToks += input
	t.llmOutputToks += output
}

// AddAgentCost records the cost reported by the agent itself.
func (t *Tracker) AddAgentCost(usd float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.agentCostUSD = usd
}

// AddSlap records a slap event.
func (t *Tracker) AddSlap() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.slapCount++
}

// PillowCost returns the estimated cost of pillow's own API usage.
func (t *Tracker) PillowCost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return float64(t.ttsChars)*t.ttsRate +
		float64(t.llmInputToks)*t.llmInputRate +
		float64(t.llmOutputToks)*t.llmOutputRate
}

// StatusLine returns a compact status string for the terminal.
func (t *Tracker) StatusLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cost := float64(t.ttsChars)*t.ttsRate +
		float64(t.llmInputToks)*t.llmInputRate +
		float64(t.llmOutputToks)*t.llmOutputRate
	return fmt.Sprintf("pillow · %d slaps · %d narrations · ~$%.3f",
		t.slapCount, t.narrationCount, cost)
}

// Summary returns a formatted session summary.
func (t *Tracker) Summary() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	duration := time.Since(t.sessionStart)
	ttsCost := float64(t.ttsChars) * t.ttsRate
	llmCost := float64(t.llmInputToks)*t.llmInputRate + float64(t.llmOutputToks)*t.llmOutputRate
	pillowTotal := ttsCost + llmCost

	return fmt.Sprintf(`
pillow session summary
──────────────────────
  Duration:     %s
  Slaps:        %d
  Narrations:   %d
  Cost breakdown:
    TTS:               $%.4f  (%d chars)
    LLM (summarizer):  $%.4f  (%d input / %d output tokens)
    Pillow total:     ~$%.4f
`,
		formatDuration(duration),
		t.slapCount,
		t.narrationCount,
		ttsCost, t.ttsChars,
		llmCost, t.llmInputToks, t.llmOutputToks,
		pillowTotal,
	)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
