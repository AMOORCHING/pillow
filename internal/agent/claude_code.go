package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"time"
)

// ClaudeCodeParser parses Claude Code's stream-json output format.
type ClaudeCodeParser struct {
	reader io.Reader
	seen   map[string]int // track content block counts per message to deduplicate
}

// NewClaudeCodeParser creates a parser that reads from the given reader.
func NewClaudeCodeParser(r io.Reader) *ClaudeCodeParser {
	return &ClaudeCodeParser{
		reader: r,
		seen:   make(map[string]int),
	}
}

// Parse reads JSON lines from the reader and emits AgentEvents.
func (p *ClaudeCodeParser) Parse(ctx context.Context, events chan<- AgentEvent) error {
	scanner := bufio.NewScanner(p.reader)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		evts := p.parseLine(line)
		for _, evt := range evts {
			select {
			case events <- evt:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return scanner.Err()
}

// rawEvent is the top-level JSON structure from Claude Code's stream.
type rawEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype"`

	// For "system" events
	SessionID string `json:"session_id"`
	Model     string `json:"model"`

	// For "assistant" events
	Message *rawMessage `json:"message"`

	// For "result" events
	Result   string  `json:"result"`
	CostUSD  float64 `json:"cost_usd"`
	Usage    *rawUsage `json:"usage"`

	// For "stream_event" events
	Event *rawStreamEvent `json:"event"`
}

type rawMessage struct {
	ID      string       `json:"id"`
	Role    string       `json:"role"`
	Content []rawContent `json:"content"`
}

type rawContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	ID    string          `json:"id"`
	Input json.RawMessage `json:"input"`
}

type rawUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type rawStreamEvent struct {
	Type         string          `json:"type"`
	Index        int             `json:"index"`
	ContentBlock *rawContent     `json:"content_block"`
	Delta        *rawDelta       `json:"delta"`
}

type rawDelta struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
}

func (p *ClaudeCodeParser) parseLine(line []byte) []AgentEvent {
	var raw rawEvent
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}

	now := time.Now()

	switch raw.Type {
	case "system":
		return []AgentEvent{{
			Type:      EventInit,
			Timestamp: now,
			SessionID: raw.SessionID,
			Model:     raw.Model,
		}}

	case "assistant":
		return p.parseAssistantMessage(raw.Message, now)

	case "result":
		evt := AgentEvent{
			Type:      EventComplete,
			Timestamp: now,
			Result:    raw.Result,
			CostUSD:   raw.CostUSD,
		}
		if raw.Usage != nil {
			evt.Usage = Usage{
				InputTokens:  raw.Usage.InputTokens,
				OutputTokens: raw.Usage.OutputTokens,
			}
		}
		return []AgentEvent{evt}

	case "stream_event":
		return p.parseStreamEvent(raw.Event, now)
	}

	return nil
}

func (p *ClaudeCodeParser) parseAssistantMessage(msg *rawMessage, now time.Time) []AgentEvent {
	if msg == nil {
		return nil
	}

	// Track how many content blocks we've already seen for this message
	prevCount := p.seen[msg.ID]
	if len(msg.Content) <= prevCount {
		return nil // no new content
	}
	p.seen[msg.ID] = len(msg.Content)

	var events []AgentEvent
	for i := prevCount; i < len(msg.Content); i++ {
		block := msg.Content[i]
		switch block.Type {
		case "text":
			if block.Text != "" {
				events = append(events, AgentEvent{
					Type:      EventText,
					Timestamp: now,
					Text:      block.Text,
				})
			}
		case "tool_use":
			evt := AgentEvent{
				Type:      EventToolUse,
				Timestamp: now,
				ToolName:  block.Name,
			}
			if block.Input != nil {
				var input map[string]any
				if err := json.Unmarshal(block.Input, &input); err == nil {
					evt.ToolInput = input
				}
			}
			events = append(events, evt)
		}
	}

	return events
}

func (p *ClaudeCodeParser) parseStreamEvent(evt *rawStreamEvent, now time.Time) []AgentEvent {
	if evt == nil {
		return nil
	}

	switch evt.Type {
	case "content_block_delta":
		if evt.Delta == nil {
			return nil
		}
		switch evt.Delta.Type {
		case "thinking_delta":
			if evt.Delta.Thinking != "" {
				return []AgentEvent{{
					Type:      EventThinking,
					Timestamp: now,
					Text:      evt.Delta.Thinking,
				}}
			}
		case "text_delta":
			if evt.Delta.Text != "" {
				return []AgentEvent{{
					Type:      EventText,
					Timestamp: now,
					Text:      evt.Delta.Text,
				}}
			}
		}
	case "content_block_start":
		if evt.ContentBlock != nil && evt.ContentBlock.Type == "tool_use" {
			return []AgentEvent{{
				Type:      EventToolUse,
				Timestamp: now,
				ToolName:  evt.ContentBlock.Name,
			}}
		}
	}

	return nil
}
