package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// ToolContext provides capabilities for the ReAct loop to interact with the repository.
type ToolContext interface {
	SearchRepoCode(query string) (string, error)
	ReadRepoFile(filePath string) (string, error)
}

type AIResponse struct {
	Issues []Finding `json:"issues"`
}

// ReviewDiff sends a diff to an OpenAI-compatible API for code review and returns findings.
func ReviewDiff(ctx ToolContext, endpoint, model, apiKey string, maxIters int, diff, agentsMD, promptExtra string) ([]Finding, error) {
	systemPrompt, userPrompt := BuildPrompt(maxIters, diff, agentsMD, promptExtra)

	config := openai.DefaultConfig(apiKey)
	if endpoint != "" {
		config.BaseURL = strings.TrimRight(endpoint, "/")
	}
	client := openai.NewClientWithConfig(config)

	tools := []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "search_repo_code",
				Description: "Search the codebase for an exact string (uses git grep under the hood).",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"query": {
							Type:        jsonschema.String,
							Description: "The exact string or regex pattern to search for in the codebase.",
						},
					},
					Required: []string{"query"},
				},
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "read_repo_file",
				Description: "Read the full contents of a specific file in the repository.",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"file_path": {
							Type:        jsonschema.String,
							Description: "The path of the file to read relative to the root of the repository.",
						},
					},
					Required: []string{"file_path"},
				},
			},
		},
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userPrompt},
	}

	var findings []Finding
	reqCtx := context.Background()
	var totalPrompt, totalCompletion int

	for iter := 0; iter < maxIters; iter++ {
		req := openai.ChatCompletionRequest{
			Model:       model,
			Messages:    messages,
			Temperature: 0.1,
			Tools:       tools,
		}

		resp, err := client.CreateChatCompletion(reqCtx, req)
		if err != nil {
			return nil, fmt.Errorf("LLM API error on iteration %d: %w", iter+1, err)
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("LLM returned no choices")
		}

		totalPrompt += resp.Usage.PromptTokens
		totalCompletion += resp.Usage.CompletionTokens

		msg := resp.Choices[0].Message
		messages = append(messages, msg)

		if len(msg.ToolCalls) == 0 {
			// No tools called, the loop finishes.
			parsed, parseErr := parseFindings(msg.Content)
			if parseErr != nil {
				return nil, fmt.Errorf("parsing findings at end of loop: %w\nRaw LLM response:\n%s", parseErr, msg.Content)
			}
			findings = parsed
			break
		}

		// Handle tool calls
		for _, tc := range msg.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    fmt.Sprintf("Error parsing arguments: %v", err),
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
				})
				continue
			}

			fmt.Printf("   🛠️ Executing tool %s...\n", tc.Function.Name)

			var toolResult string
			switch tc.Function.Name {
			case "search_repo_code":
				query, ok := args["query"].(string)
				if !ok {
					query = ""
				}
				res, err := ctx.SearchRepoCode(query)
				if err != nil {
					toolResult = fmt.Sprintf("Error executing search_repo_code: %v", err)
				} else if res == "" {
					toolResult = "No results found."
				} else {
					toolResult = res
				}
			case "read_repo_file":
				filePath, ok := args["file_path"].(string)
				if !ok {
					filePath = ""
				}
				fmt.Printf("   📄 Reading file: %s\n", filePath)
				res, err := ctx.ReadRepoFile(filePath)
				if err != nil {
					toolResult = fmt.Sprintf("Error executing read_repo_file: %v", err)
				} else if res == "" {
					toolResult = "File does not exist or is empty."
				} else {
					toolResult = res
				}
			default:
				toolResult = fmt.Sprintf("Unknown tool %s", tc.Function.Name)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    toolResult,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}

		if iter == maxIters-1 {
			fmt.Printf("⚠️ Reached max agent iterations (%d) without finishing.\n", maxIters)
		}
	}

	fmt.Printf("📊 Token usage (cumulative): %d prompt + %d completion = %d total\n",
		totalPrompt, totalCompletion, totalPrompt+totalCompletion)

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

	var aiResponse AIResponse
	if err := json.Unmarshal([]byte(content), &aiResponse); err == nil {
		return aiResponse.Issues, nil
	}

	return nil, fmt.Errorf("could not parse LLM response as findings JSON")
}
