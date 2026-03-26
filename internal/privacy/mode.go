package privacy

import (
	"fmt"

	"github.com/pillow-sh/pillow/internal/config"
	"github.com/pillow-sh/pillow/internal/narration"
	"github.com/pillow-sh/pillow/internal/tts"
)

// Components holds the summarizer and TTS provider selected by privacy mode.
type Components struct {
	Summarizer narration.Summarizer
	TTS        tts.Provider
}

// Build creates the narration components based on the privacy mode and config.
func Build(cfg *config.Config) (*Components, error) {
	mode := cfg.Privacy.Mode
	if mode == "" {
		mode = "cloud"
	}

	switch mode {
	case "cloud":
		return buildCloud(cfg)
	case "hybrid":
		return buildHybrid(cfg)
	case "local":
		return buildLocal(cfg)
	default:
		return nil, fmt.Errorf("unknown privacy mode: %s (expected: cloud, hybrid, local)", mode)
	}
}

func buildCloud(cfg *config.Config) (*Components, error) {
	// Summarizer: Haiku
	var sum narration.Summarizer
	if cfg.Narration.AnthropicAPIKey != "" {
		sum = narration.NewHaikuSummarizer(cfg.Narration.AnthropicAPIKey, cfg.Narration.Model)
	} else {
		// Fallback to local if no API key
		sum = narration.NewLocalSummarizer()
	}

	// TTS: Cartesia preferred, fallback to say
	var provider tts.Provider
	switch cfg.TTS.Provider {
	case "cartesia":
		if cfg.TTS.CartesiaAPIKey != "" {
			provider = tts.NewCartesiaProvider(cfg.TTS.CartesiaAPIKey, cfg.TTS.CartesiaVoice, cfg.TTS.CartesiaModel)
		} else {
			provider = tts.NewSayProvider("", 200)
		}
	case "piper":
		provider = tts.NewPiperProvider("")
	default:
		provider = tts.NewSayProvider("", 200)
	}

	return &Components{Summarizer: sum, TTS: provider}, nil
}

func buildHybrid(cfg *config.Config) (*Components, error) {
	// Summarizer: Haiku (cloud)
	var sum narration.Summarizer
	if cfg.Narration.AnthropicAPIKey != "" {
		sum = narration.NewHaikuSummarizer(cfg.Narration.AnthropicAPIKey, cfg.Narration.Model)
	} else {
		sum = narration.NewLocalSummarizer()
	}

	// TTS: local only
	provider := tts.Provider(tts.NewPiperProvider(""))
	return &Components{Summarizer: sum, TTS: provider}, nil
}

func buildLocal(_ *config.Config) (*Components, error) {
	return &Components{
		Summarizer: narration.NewLocalSummarizer(),
		TTS:        tts.NewSayProvider("", 200),
	}, nil
}
