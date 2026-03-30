package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level pillow configuration.
type Config struct {
	Privacy   PrivacyConfig   `toml:"privacy"`
	TTS       TTSConfig       `toml:"tts"`
	Narration NarrationConfig `toml:"narration"`
	Drift     DriftConfig     `toml:"drift"`
	Interrupt InterruptConfig `toml:"interrupt"`
	IPC       IPCConfig       `toml:"ipc"`
	Cost      CostConfig      `toml:"cost"`
}

type PrivacyConfig struct {
	Mode string `toml:"mode"` // "cloud" | "hybrid" | "local"
}

type TTSConfig struct {
	Provider       string  `toml:"provider"`         // "cartesia" | "say" | "piper"
	Speed          float64 `toml:"speed"`            // playback speed multiplier
	CartesiaAPIKey string  `toml:"cartesia_api_key"`
	CartesiaVoice  string  `toml:"cartesia_voice"`   // Cartesia voice ID
	CartesiaModel  string  `toml:"cartesia_model"`   // default "sonic-3"
}

type NarrationConfig struct {
	AnthropicAPIKey  string `toml:"anthropic_api_key"`
	Model            string `toml:"model"`              // e.g. "claude-haiku-4-5-20251001"
	Style            string `toml:"style"`              // narration style
	StaleThresholdMs int    `toml:"stale_threshold_ms"` // drop narration items older than this (default 3000)
	BatchPauseMs     int    `toml:"batch_pause_ms"`     // suppress audio if user typed recently (default 500)
	SummaryInterval  int    `toml:"summary_interval"`   // compress every N events (default 30)
}

type DriftConfig struct {
	CheckInterval int `toml:"check_interval"` // check every N tool calls (default 10)
	PauseMs       int `toml:"pause_ms"`       // or on pause > this duration (default 2000)
	CooldownS     int `toml:"cooldown_s"`     // suppress re-check after drift narration (default 30)
}

type InterruptConfig struct {
	SlapEnabled   bool    `toml:"slap_enabled"`
	SlapThreshold float64 `toml:"slap_threshold"` // g-force (default 2.5)
	SlapSound     string  `toml:"slap_sound"`     // "chime" | "pain" | "none"
	Sensitivity   float64 `toml:"sensitivity"`
	DebounceMs    int     `toml:"debounce_ms"`  // default 1000
	CooldownMs    int     `toml:"cooldown_ms"`  // default 750
	StaleMs       int     `toml:"stale_ms"`     // default 5000
}

type IPCConfig struct {
	SocketPath        string `toml:"socket_path"`         // default "/tmp/pillow.sock"
	SensordSocketPath string `toml:"sensord_socket_path"` // default "/tmp/pillowsensord.sock"
}

type CostConfig struct {
	ShowLive    bool `toml:"show_live"`
	ShowSummary bool `toml:"show_summary"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Privacy: PrivacyConfig{
			Mode: "cloud",
		},
		TTS: TTSConfig{
			CartesiaModel: "sonic-3",
		},
		Narration: NarrationConfig{
			StaleThresholdMs: 3000,
			BatchPauseMs:     500,
			SummaryInterval:  30,
		},
		Drift: DriftConfig{
			CheckInterval: 10,
			PauseMs:       2000,
			CooldownS:     30,
		},
		Interrupt: InterruptConfig{
			SlapEnabled:   true,
			SlapThreshold: 2.5,
			SlapSound:     "chime",
			DebounceMs:    1000,
			CooldownMs:    750,
			StaleMs:       5000,
		},
		IPC: IPCConfig{
			SocketPath:        "/tmp/pillow.sock",
			SensordSocketPath: "/tmp/pillowsensord.sock",
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

// HistoryPath returns the path to the history JSONL file.
func HistoryPath() string {
	return filepath.Join(ConfigDir(), "history.jsonl")
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
