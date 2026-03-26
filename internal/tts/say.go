package tts

import (
	"context"
	"os/exec"
	"sync"
)

// SayProvider uses macOS `say` command for zero-dependency TTS.
type SayProvider struct {
	voice string
	rate  int // words per minute

	mu      sync.Mutex
	current *exec.Cmd
}

// NewSayProvider creates a TTS provider using macOS say.
func NewSayProvider(voice string, rate int) *SayProvider {
	if voice == "" {
		voice = "Samantha"
	}
	if rate == 0 {
		rate = 200
	}
	return &SayProvider{voice: voice, rate: rate}
}

func (s *SayProvider) Speak(ctx context.Context, text string) error {
	cmd := exec.CommandContext(ctx, "say", "-v", s.voice, "-r", itoa(s.rate), text)

	s.mu.Lock()
	s.current = cmd
	s.mu.Unlock()

	err := cmd.Run()

	s.mu.Lock()
	s.current = nil
	s.mu.Unlock()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func (s *SayProvider) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil && s.current.Process != nil {
		_ = s.current.Process.Kill()
	}
}

func (s *SayProvider) Close() error {
	s.Stop()
	return nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 8)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
