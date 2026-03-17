# AI Reviewer

A Go CLI tool that fetches a Bitbucket Cloud Pull Request diff, sends it to an OpenAI-compatible LLM for an automated code review, and posts the findings as inline comments on the PR in Bitbucket in review mode.
This way you can review all the LLM's findings in the Bitbucket UI and approve, reject, or edit them before posting a review.

The tool only comments on added or modified lines, supports custom review directives, and can incorporate repository-specific guidelines if an `AGENTS.md` file exists in the source branch.

## Features

- **Automated Code Review**: Uses an LLM to find bugs, security vulnerabilities, and code quality issues.
- **Bitbucket Cloud Integration**: Fetches PR diffs and metadata directly from Bitbucket.
- **Smart Commenting**: Posts findings as inline comments attached to specific lines of code. Validates that findings correspond to actual additions in the PR diff.
- **Repository Context**: Automatically reads `AGENTS.md` from the PR's source branch to understand repository-specific rules.
- **Draft Comments**: Posts comments with `"pending": true` (where supported by the Bitbucket instance).
- **Dry Run Mode**: Print findings to the terminal without posting them to Bitbucket.

## Installation

Ensure you have [Go](https://go.dev/) installed, then run:

```bash
go install github.com/KellPro/ai-reviewer@latest
```

Alternatively, clone the repository and build it manually:

```bash
git clone https://github.com/KellPro/ai-reviewer.git
cd ai-reviewer
go build -o ai-reviewer .
```

## Configuration

The tool requires an LLM API key and Bitbucket credentials. You can provide these via command-line flags or environment variables.

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `AI_REVIEWER_API_KEY` | **Required.** Your OpenAI-compatible API key. | |
| `AI_REVIEWER_ENDPOINT` | The LLM API base URL. | `https://api.x.ai/v1` |
| `AI_REVIEWER_MODEL` | The LLM model to use. | `grok-code-fast-1` |
| `AI_REVIEWER_PROMPT_EXTRA` | Additional context or instructions to append to the system prompt. | |
| `BITBUCKET_EMAIL` | Your Atlassian email address (for API Token auth). | |
| `BITBUCKET_TOKEN` | A Bitbucket API Token. | |


### Authentication Methods

Go to your Atlassian Security settings and create an scoped API Token for BitBucket with the following scopes: 

- read:repository:bitbucket
- read:pullrequest:bitbucket
- write:pullrequest:bitbucket

Set `BITBUCKET_EMAIL` to your Atlassian email address, and set `BITBUCKET_TOKEN` to your API token.

## Usage

Provide the Bitbucket Cloud PR URL as the only positional argument.

```bash
# Basic usage (assuming env vars are set)
ai-reviewer https://bitbucket.org/your-workspace/your-repo/pull-requests/123

# Dry run: see the LLM's review without posting comments to Bitbucket
ai-reviewer --dry-run https://bitbucket.org/your-workspace/your-repo/pull-requests/123

# Add custom instructions for this specific review
ai-reviewer --prompt-extra "Focus heavily on SQL injection vulnerabilities" https://bitbucket.org/...

# Disable "pending" draft comments
ai-reviewer --pending=false https://bitbucket.org/...
```

### CLI Flags

Run `ai-reviewer --help` to see all available flags:

```
Usage:
  ai-reviewer <pr-url> [flags]

Flags:
      --api-key string           API key for the LLM (env: AI_REVIEWER_API_KEY)
      --bb-email string          Atlassian email address (for API Token) (env: BITBUCKET_EMAIL)
      --bb-token string          Bitbucket API Token (env: BITBUCKET_TOKEN)
      --dry-run                  Print findings without posting comments to Bitbucket
  -h, --help                     help for ai-reviewer
      --model string             Model name to use for review (default "grok-code-fast-1")
      --model-endpoint string    OpenAI-compatible API base URL (default "https://api.x.ai/v1")
      --pending                  Include "pending": true in comment payload (default true)
      --prompt-extra string      Additional review directives appended to the prompt
```

## How It Works under the Hood

1. **Parse**: Extracts the workspace, repository slug, and PR ID from the URL.
2. **Fetch Diff**: Pulls the unified diff from the Bitbucket API.
3. **Parse Diff**: Analyzes the diff to determine which files and specific lines were added or modified.
4. **Fetch AGENTS.md**: Looks for an `AGENTS.md` file in the source branch to include as repository-specific context.
5. **Review**: Constructs a prompt containing the diff, AGENTS context, and instructions, then sends it to the LLM requesting a JSON response.
6. **Validate**: Ensures the LLM's findings point to valid, added lines in the diff to prevent Bitbucket API errors.
7. **Comment**: Maps the remaining valid findings to Bitbucket inline comments and posts them to the PR.
