package tts

import "context"

// Provider is the interface for text-to-speech backends.
type Provider interface {
	// Speak converts text to audio and plays it.
	// It blocks until playback finishes or the context is cancelled.
	Speak(ctx context.Context, text string) error

	// Stop interrupts any in-progress speech immediately.
	Stop()

	// Close releases resources.
	Close() error
}
