package interrupt

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AMOORCHING/pillow/internal/bus"
)

// KeyboardListener listens for keyboard interrupt signals.
type KeyboardListener struct{}

// NewKeyboardListener creates a keyboard interrupt listener.
func NewKeyboardListener() *KeyboardListener {
	return &KeyboardListener{}
}

// Run listens for SIGQUIT (Ctrl+\) and emits keyboard interrupts.
// SIGINT (Ctrl+C) is handled by the main process for clean shutdown.
func (k *KeyboardListener) Run(ctx context.Context, interrupts chan<- bus.Interrupt) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGQUIT)
	defer signal.Stop(ch)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ch:
			select {
			case interrupts <- bus.Interrupt{
				Type:      bus.InterruptKeyboard,
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
				return
			}
		}
	}
}
