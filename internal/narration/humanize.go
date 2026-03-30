package narration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/AMOORCHING/pillow/internal/cost"
)

const humanizePrompt = `Convert this session data into one short spoken sentence (under 20 words). Casual tone, like telling a friend. No technical jargon, no token counts. Just cost and duration.

Data: %s`

// HumanizeCostForSpeech turns raw session stats into a natural spoken sentence via Haiku.
// Falls back to a template if the API call fails or no key is provided.
func HumanizeCostForSpeech(ctx context.Context, apiKey, model string, data cost.SpeechData) string {
	if apiKey == "" {
		return templateCostSummary(data)
	}
	if model == "" {
		model = "claude-haiku-4-5-20251001"
	}

	raw := fmt.Sprintf("cost: $%.4f, duration: %s, %d narrations, %d slaps",
		data.TotalCost, data.Duration.Round(time.Second), data.NarrationCount, data.SlapCount)

	reqBody := map[string]any{
		"model":      model,
		"max_tokens": 60,
		"messages": []map[string]string{
			{"role": "user", "content": fmt.Sprintf(humanizePrompt, raw)},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return templateCostSummary(data)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return templateCostSummary(data)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return templateCostSummary(data)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return templateCostSummary(data)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return templateCostSummary(data)
	}
	if len(result.Content) > 0 && result.Content[0].Text != "" {
		return result.Content[0].Text
	}
	return templateCostSummary(data)
}

func templateCostSummary(data cost.SpeechData) string {
	d := data.Duration.Round(time.Second)
	m := int(d.Minutes())

	costStr := fmt.Sprintf("%.1f cents", data.TotalCost*100)
	if data.TotalCost < 0.01 {
		costStr = "under a cent"
	}

	durStr := fmt.Sprintf("%d seconds", int(d.Seconds()))
	if m > 0 {
		durStr = fmt.Sprintf("%d minutes", m)
	}

	return fmt.Sprintf("That ran for %s and cost %s.", durStr, costStr)
}
