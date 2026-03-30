package config

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// RunWizard runs the interactive first-run setup.
func RunWizard() (*Config, error) {
	cfg := DefaultConfig()
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("  Welcome to pillow! Let's get you set up.")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Anthropic API key
	fmt.Println("  Pillow uses Haiku for summarization and drift detection.")
	fmt.Println()
	fmt.Print("  Anthropic API key (or Enter to skip): ")
	anthropicKey := readLine(reader)
	if anthropicKey != "" {
		cfg.Narration.AnthropicAPIKey = anthropicKey
		fmt.Println("  ok — Anthropic configured")
	} else {
		fmt.Println("  ok — using template-based summarization (offline)")
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Cartesia API key
	fmt.Println("  Voice narration uses Cartesia Sonic for high-quality TTS.")
	fmt.Println("  You can also run fully offline with macOS say.")
	fmt.Println()
	fmt.Print("  Cartesia API key (or Enter to skip): ")
	cartesiaKey := readLine(reader)
	if cartesiaKey != "" {
		cfg.TTS.CartesiaAPIKey = cartesiaKey
		fmt.Println("  ok — Cartesia configured")
	} else {
		fmt.Println("  ok — using macOS say (offline, lower quality)")
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Privacy mode
	fmt.Println("  Privacy — what data leaves your machine?")
	fmt.Println()
	fmt.Println("    cloud   Best quality. Summaries via Anthropic, voice via Cartesia.")
	fmt.Println("    hybrid  Summaries via Anthropic, voice generated locally.")
	fmt.Println("    local   Offline only. No API calls.")
	fmt.Println()

	// Auto-select based on available keys
	if cartesiaKey != "" && anthropicKey != "" {
		cfg.Privacy.Mode = "cloud"
		fmt.Print("  Choose [cloud/hybrid/local] (default: cloud): ")
	} else if anthropicKey != "" {
		cfg.Privacy.Mode = "hybrid"
		fmt.Print("  Choose [cloud/hybrid/local] (default: hybrid): ")
	} else {
		cfg.Privacy.Mode = "local"
		fmt.Print("  Choose [cloud/hybrid/local] (default: local): ")
	}

	mode := readLine(reader)
	if mode != "" {
		cfg.Privacy.Mode = mode
	}
	fmt.Printf("  ok — privacy mode: %s\n", cfg.Privacy.Mode)

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Voice selection
	if cartesiaKey != "" {
		fmt.Println("  Voice — choose a Cartesia voice ID.")
		fmt.Println("  Leave blank for the default voice.")
		fmt.Println()
		fmt.Print("  Voice ID (or Enter for default): ")
		voice := readLine(reader)
		if voice != "" {
			cfg.TTS.CartesiaVoice = voice
		}
		fmt.Println("  ok — voice configured")
		fmt.Println()
		fmt.Println("  ─────────────────────────────────────────────────")
		fmt.Println()
	}

	// Slap detection
	if runtime.GOARCH == "arm64" && runtime.GOOS == "darwin" {
		fmt.Println("  Slap detection uses your MacBook's accelerometer to let you")
		fmt.Println("  physically interrupt the agent. Requires pillowsensord (sudo).")
		fmt.Println()
		fmt.Print("  Enable slap detection? [Y/n]: ")
		slap := readLine(reader)
		if slap == "" || strings.ToLower(slap) == "y" || strings.ToLower(slap) == "yes" {
			fmt.Println("  ok — slap detection enabled")
		} else {
			cfg.Interrupt.SlapEnabled = false
			fmt.Println("  ok — slap detection disabled")
		}
	} else {
		cfg.Interrupt.SlapEnabled = false
		fmt.Println("  Slap detection requires Apple Silicon Mac — disabled.")
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Save config
	if err := Save(cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("  Config saved to %s\n", ConfigPath())
	fmt.Println("  Edit anytime with: pillow config")
	fmt.Println()

	return cfg, nil
}

func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
