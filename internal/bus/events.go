package bus

import (
	"time"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// InterruptType represents what triggered an interrupt.
type InterruptType int

const (
	InterruptSlap     InterruptType = iota // physical slap
	InterruptKeyboard                      // Ctrl+/ status request
)

// Interrupt is emitted when the user interrupts the agent.
type Interrupt struct {
	Type      InterruptType
	Timestamp time.Time
	Amplitude float64 // slap amplitude (for volume scaling)
}

// Bus connects all pillow components via typed channels.
type Bus struct {
	AgentEvents chan agent.AgentEvent
	Interrupts  chan Interrupt
	Done        chan struct{}
}

// New creates a new event bus with buffered channels.
func New() *Bus {
	return &Bus{
		AgentEvents: make(chan agent.AgentEvent, 64),
		Interrupts:  make(chan Interrupt, 8),
		Done:        make(chan struct{}),
	}
}
