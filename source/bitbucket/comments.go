package bitbucket

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/KellPro/ai-reviewer/source/reviewer"
)

// PostInlineComment posts a single inline comment on a pull request.
func PostInlineComment(client *Client, pr *PRInfo, finding reviewer.Finding, pending bool) error {
	apiURL := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%s/comments",
		pr.BaseURL, pr.Workspace, pr.RepoSlug, pr.PRNumber)

	severityEmoji := map[string]string{
		"error":   "🔴",
		"warning": "🟡",
		"info":    "🔵",
	}
	emoji := severityEmoji[finding.Severity]
	if emoji == "" {
		emoji = "💡"
	}

	commentBody := fmt.Sprintf("%s **AI Review (%s):** %s", emoji, finding.Severity, finding.Comment)

	payload := InlineCommentRequest{
		Content: InlineCommentContent{
			Raw: commentBody,
		},
		Inline: InlinePosition{
			Path: finding.File,
			To:   finding.Line,
		},
		Pending: pending,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling comment: %w", err)
	}

	_, err = client.Post(apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("posting comment on %s:%d: %w", finding.File, finding.Line, err)
	}

	return nil
}
