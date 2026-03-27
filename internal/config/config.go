package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level pillow configuration.
type Config struct {
	PrivacyMode string `toml:"privacy_mode"` // "cloud" | "hybrid" | "local"

	// API keys
	AnthropicAPIKey string `toml:"anthropic_api_key"`
	CartesiaAPIKey  string `toml:"cartesia_api_key"`

	// Voice
	Voice         string `toml:"voice"`          // Cartesia voice ID
	CartesiaModel string `toml:"cartesia_model"` // default "sonic-3"

	// Drift detection
	DriftCheckInterval int `toml:"drift_check_interval"` // check every N tool calls (default 10)
	DriftPauseMs       int `toml:"drift_pause_ms"`       // or on pause > this duration (default 2000)
	DriftCooldownS     int `toml:"drift_cooldown_s"`     // suppress re-check after drift narration (default 30)

	// Summarization
	SummaryInterval int `toml:"summary_interval"` // compress every N events (default 30)

	// Slap detection
	SlapThreshold  float64 `toml:"slap_threshold"`   // g-force (default 2.5)
	SlapDebounceMs int     `toml:"slap_debounce_ms"` // default 1000
	SlapStaleMs    int     `toml:"slap_stale_ms"`    // default 5000
	SlapSound      string  `toml:"slap_sound"`       // "chime" | "none" (default "chime")

	// IPC
	SocketPath        string `toml:"socket_path"`         // default "/tmp/pillow.sock"
	SensordSocketPath string `toml:"sensord_socket_path"` // default "/tmp/pillowsensord.sock"

	// Narration
	NarrationStaleMs    int `toml:"narration_stale_ms"`     // drop narration items older than this (default 3000)
	MuteWhileTypingMs   int `toml:"mute_while_typing_ms"`   // suppress audio if user typed within this window (default 500)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		PrivacyMode:   "cloud",
		CartesiaModel: "sonic-3",

		DriftCheckInterval: 10,
		DriftPauseMs:       2000,
		DriftCooldownS:     30,

		SummaryInterval: 30,

		SlapThreshold:  2.5,
		SlapDebounceMs: 1000,
		SlapStaleMs:    5000,
		SlapSound:      "chime",

		SocketPath:        "/tmp/pillow.sock",
		SensordSocketPath: "/tmp/pillowsensord.sock",

		NarrationStaleMs:  3000,
		MuteWhileTypingMs: 500,
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
		cfg.CartesiaAPIKey = key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		cfg.AnthropicAPIKey = key
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
