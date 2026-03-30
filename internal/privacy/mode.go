package privacy

import (
	"fmt"

	"github.com/AMOORCHING/pillow/internal/config"
	"github.com/AMOORCHING/pillow/internal/narration"
	"github.com/AMOORCHING/pillow/internal/tts"
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
	var sum narration.Summarizer
	if cfg.Narration.AnthropicAPIKey != "" {
		sum = narration.NewHaikuSummarizer(cfg.Narration.AnthropicAPIKey, cfg.Narration.Model)
	} else {
		sum = narration.NewLocalSummarizer()
	}

	var provider tts.Provider
	if cfg.TTS.CartesiaAPIKey != "" {
		provider = tts.NewCartesiaProvider(cfg.TTS.CartesiaAPIKey, cfg.TTS.CartesiaVoice, cfg.TTS.CartesiaModel)
	} else {
		provider = tts.NewSayProvider("", 200)
	}

	return &Components{Summarizer: sum, TTS: provider}, nil
}

func buildHybrid(cfg *config.Config) (*Components, error) {
	var sum narration.Summarizer
	if cfg.Narration.AnthropicAPIKey != "" {
		sum = narration.NewHaikuSummarizer(cfg.Narration.AnthropicAPIKey, cfg.Narration.Model)
	} else {
		sum = narration.NewLocalSummarizer()
	}

	provider := tts.Provider(tts.NewPiperProvider(""))
	return &Components{Summarizer: sum, TTS: provider}, nil
}

func buildLocal(_ *config.Config) (*Components, error) {
	return &Components{
		Summarizer: narration.NewLocalSummarizer(),
		TTS:        tts.NewSayProvider("", 200),
	}, nil
}
