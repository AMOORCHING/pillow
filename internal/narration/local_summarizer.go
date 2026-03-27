package narration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// LocalSummarizer uses template-based summarization for offline mode.
type LocalSummarizer struct{}

func NewLocalSummarizer() *LocalSummarizer {
	return &LocalSummarizer{}
}

func (l *LocalSummarizer) Summarize(_ context.Context, events []agent.AgentEvent, currentSummary string) (string, error) {
	var parts []string

	for _, evt := range events {
		switch evt.Type {
		case "preToolUse":
			parts = append(parts, l.summarizeToolUse(evt))
		case "postToolUse":
			// minimal — just note completions
		case "sessionStart":
			if evt.Goal != "" {
				parts = append(parts, fmt.Sprintf("Starting: %s.", evt.Goal))
			}
		case "sessionEnd":
			parts = append(parts, "Session complete.")
		}
	}

	if len(parts) == 0 {
		return currentSummary, nil
	}

	// Build updated summary
	if currentSummary != "" {
		// Append new activity to existing summary, keeping it bounded
		combined := currentSummary + " " + strings.Join(parts, " ")
		if len(combined) > 800 {
			combined = combined[len(combined)-800:]
		}
		return combined, nil
	}

	return strings.Join(parts, " "), nil
}

func (l *LocalSummarizer) summarizeToolUse(evt agent.AgentEvent) string {
	switch evt.Tool {
	case "Read":
		if path, ok := evt.Input["file_path"].(string); ok {
			return fmt.Sprintf("Reading %s.", filepath.Base(path))
		}
		return "Reading a file."
	case "Write":
		if path, ok := evt.Input["file_path"].(string); ok {
			return fmt.Sprintf("Creating %s.", filepath.Base(path))
		}
		return "Creating a new file."
	case "Edit":
		if path, ok := evt.Input["file_path"].(string); ok {
			return fmt.Sprintf("Editing %s.", filepath.Base(path))
		}
		return "Editing a file."
	case "Bash":
		if cmd, ok := evt.Input["command"].(string); ok {
			short := cmd
			if len(short) > 40 {
				short = short[:40] + "..."
			}
			return fmt.Sprintf("Running: %s", short)
		}
		return "Running a command."
	case "Glob":
		return "Searching for files."
	case "Grep":
		if pat, ok := evt.Input["pattern"].(string); ok {
			return fmt.Sprintf("Searching for %s.", pat)
		}
		return "Searching the codebase."
	default:
		return fmt.Sprintf("Using %s.", evt.Tool)
	}
}
