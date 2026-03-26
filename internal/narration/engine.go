package narration

import (
	"context"
	"log"
	"time"

	"github.com/pillow-sh/pillow/internal/agent"
	"github.com/pillow-sh/pillow/internal/tts"
)

// Engine orchestrates the narration pipeline: batch events → summarize → TTS.
type Engine struct {
	summarizer Summarizer
	tts        tts.Provider
	queue      *Queue

	batchPause     time.Duration
	rollingSummary string
	quiet          bool

	// OnCharsSpoken is called with the number of characters sent to TTS.
	OnCharsSpoken func(int)
}

// NewEngine creates a narration engine.
func NewEngine(summarizer Summarizer, ttsProvider tts.Provider, batchPauseMs, staleMs int) *Engine {
	if batchPauseMs == 0 {
		batchPauseMs = 500
	}
	if staleMs == 0 {
		staleMs = 3000
	}
	return &Engine{
		summarizer: summarizer,
		tts:        ttsProvider,
		queue:      NewQueue(time.Duration(staleMs) * time.Millisecond),
		batchPause: time.Duration(batchPauseMs) * time.Millisecond,
	}
}

// SetQuiet mutes audio output while still processing events.
func (e *Engine) SetQuiet(q bool) {
	e.quiet = q
}

// Run processes agent events and produces narration.
// It reads events from the channel, batches them adaptively, and speaks.
func (e *Engine) Run(ctx context.Context, events <-chan agent.AgentEvent) {
	var batch []agent.AgentEvent
	timer := time.NewTimer(e.batchPause)
	timer.Stop()

	// Speaker goroutine — reads from queue and speaks
	go e.speakerLoop(ctx)

	for {
		select {
		case <-ctx.Done():
			// Flush any remaining batch
			if len(batch) > 0 {
				e.processBatch(ctx, batch)
			}
			return

		case evt, ok := <-events:
			if !ok {
				// Channel closed — process remaining batch
				if len(batch) > 0 {
					e.processBatch(ctx, batch)
				}
				return
			}

			// Skip thinking deltas for batching (too noisy)
			if evt.Type == agent.EventThinking {
				continue
			}

			batch = append(batch, evt)

			// Natural boundaries trigger immediate processing
			if isNaturalBoundary(evt) {
				timer.Stop()
				e.processBatch(ctx, batch)
				batch = nil
				continue
			}

			// Reset the pause timer
			timer.Reset(e.batchPause)

		case <-timer.C:
			if len(batch) > 0 {
				e.processBatch(ctx, batch)
				batch = nil
			}
		}
	}
}

// SpeakImmediate queues a high-priority narration (for interrupts).
func (e *Engine) SpeakImmediate(text string) {
	e.queue.Push(text, PriorityHigh)
}

// Stop interrupts current speech.
func (e *Engine) Stop() {
	e.tts.Stop()
	e.queue.Flush()
}

func (e *Engine) processBatch(ctx context.Context, batch []agent.AgentEvent) {
	if len(batch) == 0 {
		return
	}

	text, err := e.summarizer.Summarize(ctx, batch, e.rollingSummary)
	if err != nil {
		log.Printf("summarizer error: %v", err)
		return
	}

	if text == "" {
		return
	}

	// Update rolling summary
	e.rollingSummary = text

	// Determine priority
	priority := PriorityNormal
	for _, evt := range batch {
		if evt.Type == agent.EventComplete || evt.Type == agent.EventError {
			priority = PriorityHigh
			break
		}
	}

	e.queue.Push(text, priority)
}

func (e *Engine) speakerLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		item, ok := e.queue.Pop()
		if !ok {
			// No items — wait a bit before checking again
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				continue
			}
		}

		if e.quiet {
			continue
		}

		if e.OnCharsSpoken != nil {
			e.OnCharsSpoken(len(item.Text))
		}

		if err := e.tts.Speak(ctx, item.Text); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("TTS error: %v", err)
		}
	}
}

func isNaturalBoundary(evt agent.AgentEvent) bool {
	switch evt.Type {
	case agent.EventComplete, agent.EventError, agent.EventToolResult:
		return true
	}
	return false
}
