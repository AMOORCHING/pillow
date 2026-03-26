package narration

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// LocalSummarizer uses regex/template-based summarization for offline mode.
type LocalSummarizer struct{}

func NewLocalSummarizer() *LocalSummarizer {
	return &LocalSummarizer{}
}

func (l *LocalSummarizer) Summarize(_ context.Context, events []agent.AgentEvent, _ string) (string, error) {
	var parts []string

	for _, evt := range events {
		switch evt.Type {
		case agent.EventToolUse:
			parts = append(parts, l.summarizeToolUse(evt))
		case agent.EventToolResult:
			if evt.IsError {
				parts = append(parts, "That command failed.")
			}
		case agent.EventComplete:
			parts = append(parts, "All done!")
		case agent.EventText:
			// Take first sentence
			if s := firstSentence(evt.Text); s != "" {
				parts = append(parts, s)
			}
		}
	}

	if len(parts) == 0 {
		return "", nil
	}

	// Limit to 2 sentences
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return strings.Join(parts, " "), nil
}

func (l *LocalSummarizer) summarizeToolUse(evt agent.AgentEvent) string {
	switch evt.ToolName {
	case "Read":
		if path, ok := evt.ToolInput["file_path"].(string); ok {
			return fmt.Sprintf("Reading %s.", filepath.Base(path))
		}
		return "Reading a file."
	case "Write":
		if path, ok := evt.ToolInput["file_path"].(string); ok {
			return fmt.Sprintf("Creating %s.", filepath.Base(path))
		}
		return "Creating a new file."
	case "Edit":
		if path, ok := evt.ToolInput["file_path"].(string); ok {
			return fmt.Sprintf("Editing %s.", filepath.Base(path))
		}
		return "Editing a file."
	case "Bash":
		if cmd, ok := evt.ToolInput["command"].(string); ok {
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
		if pat, ok := evt.ToolInput["pattern"].(string); ok {
			return fmt.Sprintf("Searching for %s.", pat)
		}
		return "Searching the codebase."
	default:
		return fmt.Sprintf("Using %s.", evt.ToolName)
	}
}

func firstSentence(text string) string {
	text = strings.TrimSpace(text)
	for i, r := range text {
		if r == '.' || r == '!' || r == '?' {
			s := strings.TrimSpace(text[:i+1])
			if len(s) > 100 {
				return s[:100]
			}
			return s
		}
	}
	if len(text) > 100 {
		return text[:100]
	}
	return text
}
