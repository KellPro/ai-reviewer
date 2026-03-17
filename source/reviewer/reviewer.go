package reviewer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// chatCompletionRequest is the OpenAI-compatible request body.
type chatCompletionRequest struct {
	Model       string              `json:"model"`
	Messages    []chatMessage       `json:"messages"`
	Temperature float64             `json:"temperature"`
	ResponseFormat *responseFormat  `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

// chatCompletionResponse is the OpenAI-compatible response body.
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// ReviewDiff sends a diff to an OpenAI-compatible API for code review and returns findings.
func ReviewDiff(endpoint, model, apiKey, diff, agentsMD, promptExtra string) ([]Finding, error) {
	systemPrompt, userPrompt := BuildPrompt(diff, agentsMD, promptExtra)

	reqBody := chatCompletionRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	url := strings.TrimRight(endpoint, "/") + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling LLM API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM API error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	content := chatResp.Choices[0].Message.Content
	findings, err := parseFindings(content)
	if err != nil {
		return nil, fmt.Errorf("parsing findings: %w\n\nRaw LLM response:\n%s", err, content)
	}

	fmt.Printf("📊 Token usage: %d prompt + %d completion = %d total\n",
		chatResp.Usage.PromptTokens, chatResp.Usage.CompletionTokens, chatResp.Usage.TotalTokens)

	return findings, nil
}

// parseFindings extracts a JSON array of Finding from the LLM response content.
// It handles both bare arrays and objects with a wrapping key.
func parseFindings(content string) ([]Finding, error) {
	content = strings.TrimSpace(content)

	// Strip markdown code fence if present
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		// Remove first and last lines (fences)
		if len(lines) >= 2 {
			lines = lines[1 : len(lines)-1]
			if strings.TrimSpace(lines[len(lines)-1]) == "```" {
				lines = lines[:len(lines)-1]
			}
			content = strings.Join(lines, "\n")
		}
	}

	content = strings.TrimSpace(content)

	// Try direct array parse
	var findings []Finding
	if err := json.Unmarshal([]byte(content), &findings); err == nil {
		return findings, nil
	}

	// Try single object parse (if LLM returned just one finding not in an array)
	var singleFinding Finding
	if err := json.Unmarshal([]byte(content), &singleFinding); err == nil && singleFinding.File != "" {
		return []Finding{singleFinding}, nil
	}

	// Try object wrapper (e.g. {"findings": [...]} or {"results": [...]})
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &wrapper); err == nil {
		for _, v := range wrapper {
			if err := json.Unmarshal(v, &findings); err == nil {
				return findings, nil
			}
		}
	}

	return nil, fmt.Errorf("could not parse LLM response as findings JSON")
}
