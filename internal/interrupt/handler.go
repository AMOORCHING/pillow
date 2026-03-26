package interrupt

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/AMOORCHING/pillow/internal/agent"
	"github.com/AMOORCHING/pillow/internal/bus"
	"github.com/AMOORCHING/pillow/internal/narration"
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

	// 3. Show slap feedback banner
	fmt.Println()
	fmt.Println("  ✋  slap! — agent paused")
	fmt.Println()

	// 4. Play "ow!" narration (non-blocking)
	h.engine.SpeakImmediate("Ow! Okay okay, what's wrong?")

	// 5. Prompt for redirect
	fmt.Print("  redirect → ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()
		if input != "" {
			fmt.Printf("  noted: %q\n", input)
			// TODO: inject redirect into the running agent session
		}
	}

	// 6. Resume the agent
	if err := h.bridge.Resume(); err != nil {
		log.Printf("failed to resume agent: %v", err)
	}

	fmt.Println()
	fmt.Println("  ▶  resuming...")
	fmt.Println()
}

func (h *Handler) handleKeyboard(ctx context.Context) {
	// 1. Stop current narration
	h.engine.Stop()

	// 2. Pause the agent
	if err := h.bridge.Pause(); err != nil {
		log.Printf("failed to pause agent: %v", err)
	}

	// 3. Show interrupt feedback banner
	fmt.Println()
	fmt.Println("  ⌨  ctrl+\\ — agent paused")
	fmt.Println()

	// 4. Prompt for redirect
	fmt.Print("  redirect → ")
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()
		if input != "" {
			fmt.Printf("  noted: %q\n", input)
			// TODO: inject redirect into the running agent session
		}
	}

	// 5. Resume the agent
	if err := h.bridge.Resume(); err != nil {
		log.Printf("failed to resume agent: %v", err)
	}

	fmt.Println()
	fmt.Println("  ▶  resuming...")
	fmt.Println()
}
