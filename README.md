# AI Reviewer

A Go CLI tool that can review local staged changes or fetch a Bitbucket Cloud Pull Request diff. It sends the diff to an OpenAI-compatible LLM for an automated code review, and can post findings as inline comments on the PR in Bitbucket or print them locally.
This way you can review all the LLM's findings in the Bitbucket UI and approve, reject, or edit them before posting a review.

The tool only comments on added or modified lines, supports custom review directives, and can incorporate repository-specific guidelines if an `AGENTS.md` file exists in the project.

## Features

- **Automated Code Review**: Uses an LLM to find bugs, security vulnerabilities, and code quality issues.
- **Bitbucket Cloud Integration**: Fetches PR diffs and metadata directly from Bitbucket.
- **Local Repository Support**: Review your staged changes locally before committing, or use local Git instead of the API for PR reviews.
- **Smart Commenting**: Posts findings as inline comments attached to specific lines of code. Validates that findings correspond to actual additions in the diff.
- **Repository Context**: Automatically reads `AGENTS.md` from the PR's source branch or local repository to understand repository-specific rules.
- **Draft Comments**: Posts comments with `"pending": true` (where supported by the Bitbucket instance).
- **Dry Run Mode**: Print findings to the terminal without posting them to Bitbucket (automatically enabled for local-only reviews).

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

## Quick Start

The easiest way to get started is by running the interactive `init` command:

```bash
ai-reviewer init
```

This will prompt you for your LLM API key, Bitbucket credentials, and default workspace. Non-sensitive settings are stored in `~/.config/ai-reviewer.json`. Sensitive tokens (API keys and Bitbucket tokens) are securely stored in your system's keyring.

## Configuration

The tool requires an LLM API key and Bitbucket credentials. You can provide these via command-line flags or environment variables.

### Environment Variables

| Variable | Description | Default |
|---|---|---|
| `AI_REVIEWER_API_KEY` | Your OpenAI-compatible API key. | |
| `AI_REVIEWER_ENDPOINT` | The LLM API base URL. | `https://api.x.ai/v1` |
| `AI_REVIEWER_MODEL` | The LLM model to use. | `grok-4-1-fast-reasoning` |
| `AI_REVIEWER_PROMPT_EXTRA` | Additional context or instructions to append to the system prompt. | |
| `BITBUCKET_WORKSPACE` | Default Bitbucket workspace for shorthand PR syntax. | |
| `BITBUCKET_EMAIL` | Your Atlassian email address (for API Token auth). | |
| `BITBUCKET_TOKEN` | A Bitbucket API Token. | |

### Configuration Precedence

The tool loads configuration in the following order (later values override earlier ones):
1. Internal defaults
2. Configuration file (`~/.config/ai-reviewer.json`)
3. System keyring (for `api-key` and `bb-token`)
4. Environment variables
5. Command-line flags


### Authentication Methods

Go to your Atlassian Security settings and create an scoped API Token for BitBucket with the following scopes: 

- read:repository:bitbucket
- read:pullrequest:bitbucket
- write:pullrequest:bitbucket

Set `BITBUCKET_EMAIL` to your Atlassian email address, and set `BITBUCKET_TOKEN` to your API token.

## Usage

Provide the Bitbucket Cloud PR as an argument, or run it without arguments inside a local git repository to review staged changes.

```bash
# Basic usage with full URL
ai-reviewer https://bitbucket.org/my-org/my-repo/pull-requests/123

# Shorthand usage (after running 'ai-reviewer init' or setting BB workspace)
ai-reviewer my-repo/123

# Contextual PR shorthand (if inside the PR's local git repository)
ai-reviewer 123

# Review local staged changes (dry-run mode is automatically inferred)
ai-reviewer

# Checkout the PR branch locally and review it faster using the local filesystem
ai-reviewer --switch 123

# Dry run: see the LLM's review without posting comments to Bitbucket
ai-reviewer --dry-run my-repo/123

# Add custom instructions for this specific review
ai-reviewer --prompt-extra "Focus heavily on SQL injection vulnerabilities" my-repo/123

# Disable "pending" draft comments
ai-reviewer --pending=false my-repo/123
```

### CLI Flags

Run `ai-reviewer --help` to see all available flags:

```text
Usage:
  ai-reviewer [pr-url | repo/pr-number | pr-number] [flags]
  ai-reviewer [command]

Available Commands:
  init        Configure ai-reviewer defaults and credentials
  help        Help about any command

Flags:
      --api-key string           API key for the LLM (env: AI_REVIEWER_API_KEY)
      --bb-email string          Atlassian email address (for API Token) (env: BITBUCKET_EMAIL)
      --bb-token string          Bitbucket API Token (env: BITBUCKET_TOKEN)
      --bb-workspace string      Default Bitbucket workspace (for shorthand repo/PR#)
      --dry-run                  Print findings without posting comments to Bitbucket
  -h, --help                     help for ai-reviewer
      --model string             Model name to use for review (default "grok-4-1-fast-reasoning")
      --model-endpoint string    OpenAI-compatible API base URL (default "https://api.x.ai/v1")
      --path string              Path to local repository (default ".")
      --pending                  Include "pending": true in comment payload (default true)
      --prompt-extra string      Additional review directives appended to the prompt
      --switch                   Checkout and pull PR branch locally before review (requires --path)
```

## How It Works under the Hood

1. **Resolve**: Resolves the PR reference. If a shorthand `repo/pr-number` is provided, it expands it to a full URL using the configured default workspace.
2. **Parse**: Extracts the workspace, repository slug, and PR ID from the URL.
3. **Fetch Diff**: Pulls the unified diff from the Bitbucket API.
3. **Parse Diff**: Analyzes the diff to determine which files and specific lines were added or modified.
4. **Fetch AGENTS.md**: Looks for an `AGENTS.md` file in the source branch to include as repository-specific context.
5. **Review**: Constructs a prompt containing the diff, AGENTS context, and instructions, then sends it to the LLM requesting a JSON response.
6. **Validate**: Ensures the LLM's findings point to valid, added lines in the diff to prevent Bitbucket API errors.
7. **Comment**: Maps the remaining valid findings to Bitbucket inline comments and posts them to the PR.
