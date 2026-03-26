package interrupt

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/pillow-sh/pillow/internal/agent"
	"github.com/pillow-sh/pillow/internal/bus"
	"github.com/pillow-sh/pillow/internal/narration"
)

// Handler processes interrupts and maps them to agent actions.
type Handler struct {
	bridge *agent.Bridge
	engine *narration.Engine
}

// NewHandler creates an interrupt handler.
func NewHandler(bridge *agent.Bridge, engine *narration.Engine) *Handler {
	return &Handler{bridge: bridge, engine: engine}
}

// Run processes interrupts from the bus.
func (h *Handler) Run(ctx context.Context, interrupts <-chan bus.Interrupt) {
	for {
		select {
		case <-ctx.Done():
			return
		case intr, ok := <-interrupts:
			if !ok {
				return
			}
			switch intr.Type {
			case bus.InterruptSlap:
				h.handleSlap(ctx, intr)
			case bus.InterruptKeyboard:
				h.handleKeyboard(ctx)
			}
		}
	}
}

func (h *Handler) handleSlap(ctx context.Context, intr bus.Interrupt) {
	// 1. Stop current narration
	h.engine.Stop()

	// 2. Pause the agent
	if err := h.bridge.Pause(); err != nil {
		log.Printf("failed to pause agent: %v", err)
	}

	// 3. Play "ow!" narration
	h.engine.SpeakImmediate("Ow! Okay okay, what's wrong?")

	// 4. Prompt user for input
	fmt.Print("\n> ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()
		if input != "" {
			fmt.Printf("  (noted: %s — resuming with this in mind)\n", input)
			// TODO: in a future version, inject user input as a redirect to the agent
		}
	}

	// 5. Resume the agent
	if err := h.bridge.Resume(); err != nil {
		log.Printf("failed to resume agent: %v", err)
	}

	fmt.Println("  resuming...")
}

func (h *Handler) handleKeyboard(_ context.Context) {
	h.engine.SpeakImmediate("Here's what I'm doing right now.")
}
