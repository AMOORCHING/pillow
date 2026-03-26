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
	fmt.Println("  Welcome to pillow! Let's get you set up — this takes about 30 seconds.")
	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Cartesia API key
	fmt.Println("  Voice narration needs a TTS provider. Cartesia gives the best")
	fmt.Println("  quality, but you can also run fully offline with macOS say.")
	fmt.Println()
	fmt.Print("  Do you have a Cartesia API key?\n  Paste it here (or Enter to skip): ")
	cartesiaKey := readLine(reader)
	if cartesiaKey != "" {
		cfg.TTS.Provider = "cartesia"
		cfg.TTS.CartesiaAPIKey = cartesiaKey
		fmt.Println("  ✓ Cartesia configured")
	} else {
		cfg.TTS.Provider = "say"
		fmt.Println("  ✓ Using macOS say (offline, lower quality)")
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Anthropic API key
	fmt.Println("  Pillow summarizes what the agent is doing before narrating it.")
	fmt.Println("  This uses Haiku (very cheap) for natural-sounding summaries.")
	fmt.Println()
	fmt.Print("  Paste your Anthropic API key (or Enter to skip): ")
	anthropicKey := readLine(reader)
	if anthropicKey != "" {
		cfg.Narration.AnthropicAPIKey = anthropicKey
		fmt.Println("  ✓ Anthropic configured")
	} else {
		fmt.Println("  ✓ Using template-based summarization (offline)")
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Privacy mode
	fmt.Println("  Privacy — what data leaves your machine?")
	fmt.Println()
	fmt.Println("    cloud   Best quality. Summaries → Anthropic, voice → Cartesia.")
	fmt.Println("    hybrid  Good quality. Summaries → Anthropic, voice generated locally.")
	fmt.Println("    local   Offline only. No API calls. Basic narration.")
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
	fmt.Printf("  ✓ Privacy mode: %s\n", cfg.Privacy.Mode)

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Slap detection
	if runtime.GOARCH == "arm64" && runtime.GOOS == "darwin" {
		fmt.Println("  Slap detection uses your MacBook's accelerometer to let you")
		fmt.Println("  physically interrupt the agent. Requires a background daemon (sudo).")
		fmt.Println()
		fmt.Print("  Enable slap detection? [Y/n]: ")
		slap := readLine(reader)
		if slap == "" || strings.ToLower(slap) == "y" || strings.ToLower(slap) == "yes" {
			cfg.Interrupt.SlapEnabled = true
			fmt.Println("  ✓ Slap detection enabled")
		} else {
			cfg.Interrupt.SlapEnabled = false
			fmt.Println("  ✓ Slap detection disabled (use Ctrl+\\ for keyboard interrupt)")
		}
	} else {
		cfg.Interrupt.SlapEnabled = false
		fmt.Println("  Slap detection requires Apple Silicon Mac — using keyboard interrupts.")
	}

	fmt.Println()
	fmt.Println("  ─────────────────────────────────────────────────")
	fmt.Println()

	// Save config
	if err := Save(cfg); err != nil {
		return nil, fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("  ✓ Config saved to %s\n", ConfigPath())
	fmt.Println("    Edit anytime with: pillow config")
	fmt.Println()

	return cfg, nil
}

func readLine(reader *bufio.Reader) string {
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}
