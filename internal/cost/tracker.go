package cost

import (
	"fmt"
	"sync"
	"time"
)

// Default rates (as of 2025)
const (
	CartesiaRatePerChar   = 0.000006
	HaikuInputRatePerTok  = 0.0000008
	HaikuOutputRatePerTok = 0.000004
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

	// Drift detection costs
	driftInputToks  int
	driftOutputToks int

	sessionStart   time.Time
	slapCount      int
	narrationCount int
}

// NewTracker creates a cost tracker with default rates.
func NewTracker() *Tracker {
	return &Tracker{
		ttsRate:       CartesiaRatePerChar,
		llmInputRate:  HaikuInputRatePerTok,
		llmOutputRate: HaikuOutputRatePerTok,
		sessionStart:  time.Now(),
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

// AddDriftTokens records tokens used by drift detection.
func (t *Tracker) AddDriftTokens(input, output int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.driftInputToks += input
	t.driftOutputToks += output
}

// AddSlap records a slap event.
func (t *Tracker) AddSlap() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.slapCount++
}

// EstimateCost returns the total estimated cost.
func (t *Tracker) EstimateCost() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return float64(t.ttsChars)*t.ttsRate +
		float64(t.llmInputToks)*t.llmInputRate +
		float64(t.llmOutputToks)*t.llmOutputRate +
		float64(t.driftInputToks)*t.llmInputRate +
		float64(t.driftOutputToks)*t.llmOutputRate
}

// StatusLine returns a compact status string.
func (t *Tracker) StatusLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	cost := float64(t.ttsChars)*t.ttsRate +
		float64(t.llmInputToks+t.driftInputToks)*t.llmInputRate +
		float64(t.llmOutputToks+t.driftOutputToks)*t.llmOutputRate
	return fmt.Sprintf("~$%.4f (%d narrations, %d slaps)", cost, t.narrationCount, t.slapCount)
}

// Summary returns a formatted session cost summary.
func (t *Tracker) Summary() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	duration := time.Since(t.sessionStart)
	ttsCost := float64(t.ttsChars) * t.ttsRate
	summarizerCost := float64(t.llmInputToks)*t.llmInputRate + float64(t.llmOutputToks)*t.llmOutputRate
	driftCost := float64(t.driftInputToks)*t.llmInputRate + float64(t.driftOutputToks)*t.llmOutputRate
	total := ttsCost + summarizerCost + driftCost

	return fmt.Sprintf(
		"Session cost: ~$%.4f (summarizer: %d tokens, drift: %d tokens, TTS: %d chars) over %s",
		total,
		t.llmInputToks+t.llmOutputToks,
		t.driftInputToks+t.driftOutputToks,
		t.ttsChars,
		formatDuration(duration),
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
