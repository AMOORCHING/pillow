package narration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// Summarizer converts agent events into narration text.
type Summarizer interface {
	Summarize(ctx context.Context, events []agent.AgentEvent, rollingSummary string) (string, error)
}

// HaikuSummarizer uses Anthropic's Haiku model for natural narration.
type HaikuSummarizer struct {
	apiKey string
	model  string

	// Cost tracking callbacks
	OnTokensUsed func(input, output int)
}

// NewHaikuSummarizer creates a summarizer using Anthropic's API.
func NewHaikuSummarizer(apiKey, model string) *HaikuSummarizer {
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}
	return &HaikuSummarizer{apiKey: apiKey, model: model}
}

const systemPrompt = `You are narrating what a coding agent is doing in real-time. Be concise, casual, and technical. Speak as if you're a coworker walking someone through code changes. Never read code verbatim — describe what's happening at a conceptual level.

Rules:
- Generate 1-2 sentences of narration max
- Be specific about file names and what's changing
- Don't read code line by line
- Use natural spoken language (this will be read aloud)
- Don't use markdown, bullet points, or formatting
- Don't start with "The agent" — speak as the agent ("I'm creating...", "Let me look at...")`

func (h *HaikuSummarizer) Summarize(ctx context.Context, events []agent.AgentEvent, rollingSummary string) (string, error) {
	eventsDesc := formatEventsForLLM(events)

	userMsg := ""
	if rollingSummary != "" {
		userMsg = fmt.Sprintf("Recent context: %s\n\nNew events:\n%s", rollingSummary, eventsDesc)
	} else {
		userMsg = fmt.Sprintf("New events:\n%s", eventsDesc)
	}

	reqBody := map[string]any{
		"model":      h.model,
		"max_tokens": 100,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userMsg},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", h.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if h.OnTokensUsed != nil {
		h.OnTokensUsed(result.Usage.InputTokens, result.Usage.OutputTokens)
	}

	if len(result.Content) > 0 {
		return result.Content[0].Text, nil
	}
	return "", nil
}

func formatEventsForLLM(events []agent.AgentEvent) string {
	var buf bytes.Buffer
	for _, evt := range events {
		switch evt.Type {
		case agent.EventThinking:
			text := evt.Text
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			fmt.Fprintf(&buf, "- Thinking: %s\n", text)
		case agent.EventText:
			text := evt.Text
			if len(text) > 200 {
				text = text[:200] + "..."
			}
			fmt.Fprintf(&buf, "- Said: %s\n", text)
		case agent.EventToolUse:
			fmt.Fprintf(&buf, "- Tool: %s", evt.ToolName)
			if path, ok := evt.ToolInput["file_path"]; ok {
				fmt.Fprintf(&buf, " (file: %v)", path)
			}
			if cmd, ok := evt.ToolInput["command"]; ok {
				cmdStr := fmt.Sprint(cmd)
				if len(cmdStr) > 80 {
					cmdStr = cmdStr[:80] + "..."
				}
				fmt.Fprintf(&buf, " (command: %v)", cmdStr)
			}
			buf.WriteByte('\n')
		case agent.EventToolResult:
			if evt.IsError {
				fmt.Fprintf(&buf, "- Tool error: %s\n", truncate(evt.Stderr, 100))
			} else {
				fmt.Fprintf(&buf, "- Tool completed successfully\n")
			}
		case agent.EventComplete:
			fmt.Fprintf(&buf, "- Task completed\n")
		case agent.EventError:
			fmt.Fprintf(&buf, "- Error: %s\n", evt.Text)
		}
	}
	return buf.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
