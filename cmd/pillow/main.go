package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/AMOORCHING/pillow/internal/agent"
	"github.com/AMOORCHING/pillow/internal/bus"
	"github.com/AMOORCHING/pillow/internal/config"
	"github.com/AMOORCHING/pillow/internal/cost"
	"github.com/AMOORCHING/pillow/internal/interrupt"
	"github.com/AMOORCHING/pillow/internal/narration"
	"github.com/AMOORCHING/pillow/internal/privacy"
)

var (
	flagQuiet      bool
	flagNoSlap     bool
	flagPrivacy    string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "pillow <agent> [prompt]",
		Short: "Voice-narrated agentic coding with physical interrupts",
		Long: `pillow wraps agentic coding tools (Claude Code) and adds real-time
voice narration of what the agent is doing, plus physical interrupt
support via MacBook accelerometer (slap to pause).`,
		Args:                  cobra.MinimumNArgs(1),
		DisableFlagsInUseLine: true,
		RunE:                  runAgent,
	}

	rootCmd.Flags().BoolVar(&flagQuiet, "quiet", false, "mute voice narration")
	rootCmd.Flags().BoolVar(&flagQuiet, "no-voice", false, "mute voice narration (alias for --quiet)")
	rootCmd.Flags().BoolVar(&flagNoSlap, "no-slap", false, "disable accelerometer, keyboard only")
	rootCmd.Flags().StringVar(&flagPrivacy, "privacy", "", "override privacy mode (cloud/hybrid/local)")

	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Run the interactive setup wizard",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := config.RunWizard()
			return err
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Open config file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ConfigPath()
			if !config.Exists() {
				fmt.Printf("No config file found. Run 'pillow setup' first, or create %s manually.\n", path)
				return nil
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "nano"
			}
			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}

	sensordCmd := &cobra.Command{
		Use:   "sensord",
		Short: "Manage the sensor daemon",
	}

	sensordStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "Check if sensord is running",
		Run: func(cmd *cobra.Command, args []string) {
			if interrupt.SensordRunning("") {
				fmt.Println("pillowsensord is running")
			} else {
				fmt.Println("pillowsensord is not running")
				fmt.Println("Start with: sudo pillowsensord")
			}
		},
	}

	sensordCmd.AddCommand(sensordStatusCmd)
	rootCmd.AddCommand(setupCmd, configCmd, sensordCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runAgent(cmd *cobra.Command, args []string) error {
	agentName := args[0]
	prompt := ""
	if len(args) > 1 {
		prompt = args[1]
	}

	// Load config (run wizard if first time)
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if !config.Exists() {
		fmt.Println("  First time? Let's set up pillow.")
		wizardCfg, err := config.RunWizard()
		if err != nil {
			return err
		}
		cfg = wizardCfg
		fmt.Println("  Starting your session now...")
		fmt.Println()
	}

	// Override privacy mode if flag is set
	if flagPrivacy != "" {
		cfg.Privacy.Mode = flagPrivacy
	}

	// Build components based on privacy mode
	components, err := privacy.Build(cfg)
	if err != nil {
		return err
	}
	defer components.TTS.Close()

	// Create cost tracker
	tracker := cost.NewTracker()

	// Create narration engine
	engine := narration.NewEngine(
		components.Summarizer,
		components.TTS,
		cfg.Narration.BatchPauseMs,
		cfg.Narration.StaleThresholdMs,
	)
	engine.SetQuiet(flagQuiet)
	engine.OnCharsSpoken = func(n int) {
		tracker.AddTTSChars(n)
	}

	// Wire up LLM cost tracking if using Haiku summarizer
	if hs, ok := components.Summarizer.(*narration.HaikuSummarizer); ok {
		hs.OnTokensUsed = func(input, output int) {
			tracker.AddLLMTokens(input, output)
		}
	}

	// Create agent bridge
	bridge, err := agent.NewBridge(agentName, prompt)
	if err != nil {
		return err
	}

	// Create event bus
	eventBus := bus.New()

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n  shutting down...")
		cancel()
		bridge.Signal(syscall.SIGINT)
	}()

	// Start keyboard interrupt listener
	go interrupt.NewKeyboardListener().Run(ctx, eventBus.Interrupts)

	// Start accelerometer listener (if enabled and available)
	if !flagNoSlap && cfg.Interrupt.SlapEnabled {
		if interrupt.SensordRunning("") {
			go func() {
				client := interrupt.NewAccelClient("")
				if err := client.Run(ctx, eventBus.Interrupts); err != nil {
					log.Printf("accelerometer: %v", err)
				}
			}()
		} else {
			fmt.Println("  note: slap detection unavailable (pillowsensord not running)")
			fmt.Println("  use Ctrl+\\ for keyboard interrupt, or start with: sudo pillowsensord")
		}
	}

	// Start interrupt handler
	interruptHandler := interrupt.NewHandler(bridge, engine)
	go interruptHandler.Run(ctx, eventBus.Interrupts)

	// Start narration engine (reads from a copy of agent events)
	narrationEvents := make(chan agent.AgentEvent, 64)
	go engine.Run(ctx, narrationEvents)

	// Fan out: agent events go to both narration and cost tracking
	go func() {
		for evt := range eventBus.AgentEvents {
			// Forward to narration
			select {
			case narrationEvents <- evt:
			default:
				// Drop if narration is backed up
			}

			// Track costs from agent completion events
			if evt.Type == agent.EventComplete && evt.CostUSD > 0 {
				tracker.AddAgentCost(evt.CostUSD)
			}
		}
		close(narrationEvents)
	}()

	// Print session start
	if !flagQuiet {
		fmt.Printf("  🎙 pillow · %s mode", cfg.Privacy.Mode)
		if cfg.Interrupt.SlapEnabled && !flagNoSlap {
			fmt.Print(" · slap enabled")
		}
		fmt.Println()
		fmt.Println()
	}

	// Run the agent (blocks until completion)
	err = bridge.Run(ctx, eventBus.AgentEvents)
	close(eventBus.AgentEvents)

	// Print session summary
	if cfg.Cost.ShowSummary {
		fmt.Print(tracker.Summary())
	}

	return err
}
