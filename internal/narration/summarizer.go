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
	Summarize(ctx context.Context, events []agent.AgentEvent, currentSummary string) (string, error)
}

// HaikuSummarizer uses Anthropic's Haiku model for rolling compression.
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

const rollingSummaryPrompt = `You are summarizing an agentic coding session for a developer who stepped away. Here is the running summary so far:

%s

Here are the %d new events since the last summary:

%s

Update the summary. Rules:
- Narrative voice, as if briefing a colleague. Not a list.
- Focus on: what changed, what broke, what decisions the agent made, what's in progress now.
- Mention any warnings or interruptions that occurred.
- Max 200 words.
- This will be read aloud via TTS, so write for the ear, not the eye.`

func (h *HaikuSummarizer) Summarize(ctx context.Context, events []agent.AgentEvent, currentSummary string) (string, error) {
	eventsDesc := formatEventsForLLM(events)

	userMsg := fmt.Sprintf(rollingSummaryPrompt, currentSummary, len(events), eventsDesc)

	reqBody := map[string]any{
		"model":      h.model,
		"max_tokens": 400,
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
		case "preToolUse":
			fmt.Fprintf(&buf, "- Tool: %s", evt.Tool)
			if path, ok := evt.Input["file_path"]; ok {
				fmt.Fprintf(&buf, " (file: %v)", path)
			}
			if cmd, ok := evt.Input["command"]; ok {
				cmdStr := fmt.Sprint(cmd)
				if len(cmdStr) > 80 {
					cmdStr = cmdStr[:80] + "..."
				}
				fmt.Fprintf(&buf, " (command: %v)", cmdStr)
			}
			buf.WriteByte('\n')
		case "postToolUse":
			if evt.Output != "" {
				output := evt.Output
				if len(output) > 100 {
					output = output[:100] + "..."
				}
				fmt.Fprintf(&buf, "- Tool result: %s\n", output)
			} else {
				fmt.Fprintf(&buf, "- Tool completed\n")
			}
		case "sessionStart":
			fmt.Fprintf(&buf, "- Session started (goal: %s)\n", evt.Goal)
		case "sessionEnd":
			fmt.Fprintf(&buf, "- Session ended\n")
		}
	}
	return buf.String()
}
