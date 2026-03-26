package tts

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// PiperProvider uses local Piper TTS for offline speech synthesis.
type PiperProvider struct {
	model string

	mu      sync.Mutex
	current *exec.Cmd
}

// NewPiperProvider creates a Piper TTS provider.
func NewPiperProvider(model string) *PiperProvider {
	if model == "" {
		model = "en_US-lessac-medium"
	}
	return &PiperProvider{model: model}
}

func (p *PiperProvider) Speak(ctx context.Context, text string) error {
	// Check if piper is available
	if _, err := exec.LookPath("piper"); err != nil {
		return fmt.Errorf("piper not found in PATH — install with: brew install piper")
	}

	cmd := exec.CommandContext(ctx, "piper",
		"--model", p.model,
		"--output-raw",
	)
	cmd.Stdin = strings.NewReader(text)

	// Capture raw PCM output
	pcmData, err := func() ([]byte, error) {
		p.mu.Lock()
		p.current = cmd
		p.mu.Unlock()

		out, err := cmd.Output()

		p.mu.Lock()
		p.current = nil
		p.mu.Unlock()

		return out, err
	}()

	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("piper TTS: %w", err)
	}

	// Piper outputs PCM s16le at 22050Hz by default
	return playPiperPCM(ctx, pcmData)
}

func (p *PiperProvider) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current != nil && p.current.Process != nil {
		_ = p.current.Process.Kill()
	}
}

func (p *PiperProvider) Close() error {
	p.Stop()
	return nil
}

func playPiperPCM(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}

	// Try sox play first (22050Hz for piper)
	cmd := exec.CommandContext(ctx, "play",
		"-t", "raw", "-r", "22050", "-b", "16", "-e", "signed", "-c", "1", "-")
	cmd.Stdin = newBytesReader(data)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback to afplay via WAV
	wav := buildWAV(data, 22050, 16, 1)
	f, err := createTempFile("pillow-piper-*.wav")
	if err != nil {
		return err
	}
	defer removeFile(f)

	if _, err := f.Write(wav); err != nil {
		f.Close()
		return err
	}
	f.Close()

	playCmd := exec.CommandContext(ctx, "afplay", f.Name())
	return playCmd.Run()
}
