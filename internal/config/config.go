package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level pillow configuration.
type Config struct {
	TTS       TTSConfig       `toml:"tts"`
	Narration NarrationConfig `toml:"narration"`
	Interrupt InterruptConfig `toml:"interrupt"`
	Privacy   PrivacyConfig   `toml:"privacy"`
	Cost      CostConfig      `toml:"cost"`
}

type TTSConfig struct {
	Provider string `toml:"provider"` // cartesia, piper, say
	Speed    float64 `toml:"speed"`

	// Cartesia settings
	CartesiaAPIKey string `toml:"cartesia_api_key"`
	CartesiaVoice  string `toml:"cartesia_voice"`
	CartesiaModel  string `toml:"cartesia_model"`
}

type NarrationConfig struct {
	Style              string `toml:"style"` // default, minimal, verbose
	Model              string `toml:"model"` // summarizer model
	AnthropicAPIKey    string `toml:"anthropic_api_key"`
	StaleThresholdMs   int    `toml:"stale_threshold_ms"`
	BatchPauseMs       int    `toml:"batch_pause_ms"`
}

type InterruptConfig struct {
	SlapEnabled bool    `toml:"slap_enabled"`
	SlapSound   string  `toml:"slap_sound"` // pain, sexy, halo
	Sensitivity float64 `toml:"sensitivity"`
	CooldownMs  int     `toml:"cooldown_ms"`
}

type PrivacyConfig struct {
	Mode string `toml:"mode"` // cloud, hybrid, local
}

type CostConfig struct {
	ShowLive    bool `toml:"show_live"`
	ShowSummary bool `toml:"show_summary"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		TTS: TTSConfig{
			Provider:      "say",
			Speed:         1.0,
			CartesiaModel: "sonic-3",
		},
		Narration: NarrationConfig{
			Style:            "default",
			Model:            "claude-haiku-4-5-20251001",
			StaleThresholdMs: 3000,
			BatchPauseMs:     500,
		},
		Interrupt: InterruptConfig{
			SlapEnabled: false,
			SlapSound:   "pain",
			Sensitivity: 0.15,
			CooldownMs:  750,
		},
		Privacy: PrivacyConfig{
			Mode: "cloud",
		},
		Cost: CostConfig{
			ShowLive:    true,
			ShowSummary: true,
		},
	}
}

// ConfigDir returns the pillow config directory path.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "pillow")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "pillow")
}

// ConfigPath returns the path to the config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// Load reads the config file, falling back to defaults.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	// Environment variables override config file
	if key := os.Getenv("CARTESIA_API_KEY"); key != "" {
		cfg.TTS.CartesiaAPIKey = key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.Narration.AnthropicAPIKey = key
	}

	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	f, err := os.Create(ConfigPath())
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	return enc.Encode(cfg)
}

// Exists returns true if a config file exists.
func Exists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}
