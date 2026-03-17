package reviewer

import (
	"fmt"
	"strings"
)

// BuildPrompt constructs the system and user messages for the LLM code review.
func BuildPrompt(diff, agentsMD, promptExtra string) (system, user string) {
	var sb strings.Builder

	sb.WriteString(`You are an expert code reviewer. You will be given a unified diff from a pull request.
Your task is to carefully review the ADDED and MODIFIED lines for:
- Bugs and logic errors
- Security vulnerabilities
- Performance issues
- Error handling problems
- Code quality issues (but be pragmatic, not pedantic)

IMPORTANT RULES:
1. ONLY comment on lines that appear as additions (lines starting with "+") in the diff.
2. Be concise and actionable. Each comment should clearly explain the issue and suggest a fix.
3. Do NOT comment on style-only issues like formatting, naming conventions, or whitespace unless they cause bugs.
4. Do NOT comment on deleted lines.
5. If you find no issues, return an empty JSON array: []

Return your findings as a JSON array with the following structure:
` + "```json\n" + `[
  {
    "file": "path/to/file.ext",
    "line": 42,
    "severity": "error",
    "comment": "Description of the issue and suggested fix"
  },
	{
    "file": "path/to/file.ext",
    "line": 27,
    "severity": "warning",
    "comment": "Description of the issue and suggested fix"
  }, // etc, until all issues are addressed
]
` + "```\n" + `
Severity levels:
- "error": Bugs, security vulnerabilities, data loss risks
- "warning": Potential issues, race conditions, missing error handling
- "info": Suggestions for improvement, minor issues

Return ONLY the JSON array. No other text, explanation, or markdown formatting around it.`)

	if promptExtra != "" {
		sb.WriteString("\n\nADDITIONAL REVIEW DIRECTIVES:\n")
		sb.WriteString(promptExtra)
	}

	system = sb.String()

	// Build user message with diff and optional AGENTS.md
	var userSB strings.Builder
	if agentsMD != "" {
		userSB.WriteString("## Repository Review Guidelines (AGENTS.md)\n\n")
		userSB.WriteString(agentsMD)
		userSB.WriteString("\n\n---\n\n")
	}
	userSB.WriteString(fmt.Sprintf("## Pull Request Diff\n\n```diff\n%s\n```", diff))

	user = userSB.String()

	return system, user
}
