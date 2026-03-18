package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/KellPro/ai-reviewer/source/bitbucket"
	"github.com/KellPro/ai-reviewer/source/config"
	"github.com/KellPro/ai-reviewer/source/parser"
	"github.com/KellPro/ai-reviewer/source/provider"
	"github.com/KellPro/ai-reviewer/source/reviewer"
)

// shorthandRe matches "repo/123" (a repo slug and a PR number separated by a slash).
var shorthandRe = regexp.MustCompile(`^([a-zA-Z0-9._-]+)/(\d+)$`)

func main() {
	cfg := config.DefaultConfig()

	rootCmd := &cobra.Command{
		Use:   "ai-reviewer [pr-url | repo/pr-number]",
		Short: "AI-powered Bitbucket PR code reviewer",
		Long: `ai-reviewer fetches the diff from a Bitbucket Cloud pull request,
sends it to an OpenAI-compatible LLM for code review, and posts
the findings as inline comments on the PR in review mode.

Authentication is via Bitbucket API Tokens.

You can use a full PR URL or a shorthand "repo/pr-number" when
a default workspace has been configured via 'ai-reviewer init'.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return run(cfg, "")
			}
			prArg := args[0]

			// If it's a simple number and a path is specified (or default .), try to infer workspace/repo
			if matched, _ := regexp.MatchString(`^\d+$`, prArg); matched && provider.IsGitRepo(cfg.Path) {
				workspace, repo, err := provider.GetBitbucketRepoFromPath(cfg.Path)
				if err == nil {
					prURL := fmt.Sprintf("https://bitbucket.org/%s/%s/pull-requests/%s", workspace, repo, prArg)
					return run(cfg, prURL)
				}
			}

			// Resolve shorthand "repo/123" → full URL
			prURL, err := resolvePRArg(prArg, cfg.BBWorkspace)
			if err != nil {
				return err
			}

			return run(cfg, prURL)
		},
	}

	// Init subcommand
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Configure ai-reviewer defaults and credentials",
		Long: `Interactively configure ai-reviewer. Non-sensitive settings are
stored in ~/.config/ai-reviewer.json. Secrets (API key, Bitbucket
token) are stored in your system keyring.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return config.RunInit()
		},
	}
	rootCmd.AddCommand(initCmd)

	flags := rootCmd.Flags()
	flags.StringVar(&cfg.ModelEndpoint, "model-endpoint", cfg.ModelEndpoint, "OpenAI-compatible API base URL")
	flags.StringVar(&cfg.Model, "model", cfg.Model, "Model name to use for review")
	flags.StringVar(&cfg.APIKey, "api-key", cfg.APIKey, "API key for the LLM (env: AI_REVIEWER_API_KEY)")
	flags.StringVar(&cfg.PromptExtra, "prompt-extra", cfg.PromptExtra, "Additional review directives appended to the prompt")
	flags.StringVar(&cfg.BBWorkspace, "bb-workspace", cfg.BBWorkspace, "Default Bitbucket workspace (for shorthand repo/PR#)")
	flags.StringVar(&cfg.BBEmail, "bb-email", cfg.BBEmail, "Atlassian email address (for API Token) (env: BITBUCKET_EMAIL)")
	flags.StringVar(&cfg.BBToken, "bb-token", cfg.BBToken, "Bitbucket API Token (env: BITBUCKET_TOKEN)")
	flags.StringVar(&cfg.Path, "path", cfg.Path, "Path to local repository (default: .)")
	flags.BoolVar(&cfg.Switch, "switch", cfg.Switch, "Checkout and pull PR branch locally before review (requires --path)")
	flags.BoolVar(&cfg.Pending, "pending", cfg.Pending, "Include \"pending\": true in comment payload")
	flags.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Print findings without posting comments to Bitbucket")

	// Hide sensitive defaults from help output
	flags.Lookup("api-key").DefValue = `********`
	flags.Lookup("bb-token").DefValue = `********`

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// resolvePRArg expands a shorthand "repo/123" to a full Bitbucket PR URL,
// or returns the argument as-is if it's already a URL.
func resolvePRArg(arg, defaultWorkspace string) (string, error) {
	if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
		return arg, nil
	}

	matches := shorthandRe.FindStringSubmatch(arg)
	if matches == nil {
		return "", fmt.Errorf("invalid PR reference: %q\nUse a full URL or shorthand \"repo/pr-number\" (requires --bb-workspace or 'ai-reviewer init')", arg)
	}

	if defaultWorkspace == "" {
		return "", fmt.Errorf("shorthand %q requires a default workspace\nSet it with --bb-workspace, BITBUCKET_WORKSPACE env var, or run 'ai-reviewer init'", arg)
	}

	repo := matches[1]
	prNum := matches[2]
	return fmt.Sprintf("https://bitbucket.org/%s/%s/pull-requests/%s", defaultWorkspace, repo, prNum), nil
}

func run(cfg *config.Config, prURL string) error {
	// Validate config
	if err := cfg.Validate(); err != nil {
		return err
	}

	var ctx provider.ReviewContext
	var diff string
	var err error
	var prInfo *bitbucket.PRInfo
	var client *bitbucket.Client

	if prURL == "" {
		if !provider.IsGitRepo(cfg.Path) {
			return fmt.Errorf("no arguments supplied and path '%s' is not a git repository", cfg.Path)
		}
		fmt.Printf("🔍 Running local staged review in %s (dry-run mode)...\n", cfg.Path)
		cfg.DryRun = true
		ctx = provider.NewGitStagedContext(cfg.Path)

		fmt.Printf("📄 Fetching staged diff...\n")
		diff, err = ctx.GetDiff()
		if err != nil {
			return fmt.Errorf("fetching diff: %w", err)
		}
	} else {
		// Parse PR URL
		fmt.Printf("🔗 Parsing PR URL: %s\n", prURL)
		prInfo, err = bitbucket.ParsePRURL(prURL)
		if err != nil {
			return err
		}
		fmt.Printf("   Workspace: %s, Repo: %s, PR #%s\n", prInfo.Workspace, prInfo.RepoSlug, prInfo.PRNumber)

		// Authenticate
		fmt.Printf("🔐 Authenticating with Bitbucket...\n")
		authHeader, err := bitbucket.Authenticate(cfg.BBEmail, cfg.BBToken)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
		client = bitbucket.NewClient(authHeader)
		fmt.Printf("   ✅ Authenticated successfully\n")

		// Fetch PR metadata
		fmt.Printf("📋 Fetching PR metadata...\n")
		meta, err := bitbucket.GetPRMetadata(client, prInfo)
		if err != nil {
			return fmt.Errorf("fetching PR metadata: %w", err)
		}
		sourceRef := meta.Source.Commit.Hash
		sourceRepo := meta.Source.Repository.FullName
		fmt.Printf("   Title: %s\n", meta.Title)
		fmt.Printf("   Branch: %s → %s\n", meta.Source.Branch.Name, meta.Destination.Branch.Name)

		var remote string
		if provider.IsGitRepo(cfg.Path) {
			remote, _ = provider.GetRemoteName(cfg.Path, prInfo.Workspace, prInfo.RepoSlug)
		}

		if remote != "" {
			fmt.Printf("🏠 Local repository matches PR. Using local Git provider (remote: %s).\n", remote)
			if cfg.Switch {
				fmt.Printf("🔀 Switching to branch '%s' and pulling from %s...\n", meta.Source.Branch.Name, remote)
				if err := provider.SwitchAndPull(cfg.Path, meta.Source.Branch.Name, remote); err != nil {
					return fmt.Errorf("failed to switch and pull branch %s: %w", meta.Source.Branch.Name, err)
				}
			}
			ctx = provider.NewGitPRContext(cfg.Path, meta.Destination.Commit.Hash, sourceRef)
		} else {
			ctx = provider.NewBitbucketContext(client, prInfo, sourceRef, sourceRepo)
		}

		// Fetch diff
		fmt.Printf("📄 Fetching PR diff...\n")
		diff, err = ctx.GetDiff()
		if err != nil {
			return fmt.Errorf("fetching diff: %w", err)
		}
	}

	diffFiles := parser.ParseUnifiedDiff(diff)
	totalAddedLines := 0
	for _, f := range diffFiles {
		for _, h := range f.Hunks {
			for _, l := range h.Lines {
				if l.Type == "add" {
					totalAddedLines++
				}
			}
		}
	}
	fmt.Printf("   %d files changed, %d lines added\n", len(diffFiles), totalAddedLines)

	if len(diffFiles) == 0 {
		fmt.Printf("⚠️  No changes found in the diff. Nothing to review.\n")
		return nil
	}

	fmt.Printf("📖 Looking for AGENTS.md...\n")
	var agentsMD string
	possibleNames := []string{"AGENTS.md", "agents.md"}
	for _, name := range possibleNames {
		content, err := ctx.GetFileContent(name)
		if err == nil && content != "" {
			agentsMD = content
			fmt.Printf("   ✅ Found %s (%d bytes)\n", name, len(agentsMD))
			break
		} else if err != nil {
			fmt.Printf("   ⚠️  Could not fetch %s: %v\n", name, err)
		}
	}
	if agentsMD == "" {
		fmt.Printf("   ℹ️  No AGENTS.md found\n")
	}

	// Send to LLM for review
	fmt.Printf("🤖 Sending diff to %s (model: %s)...\n", cfg.ModelEndpoint, cfg.Model)
	findings, err := reviewer.ReviewDiff(cfg.ModelEndpoint, cfg.Model, cfg.APIKey, diff, agentsMD, cfg.PromptExtra)
	if err != nil {
		return fmt.Errorf("LLM review failed: %w", err)
	}
	fmt.Printf("   Found %d issue(s)\n", len(findings))

	if len(findings) == 0 {
		fmt.Printf("✅ No issues found! The code looks good.\n")
		return nil
	}

	// Validate findings against diff lines
	validLines := parser.ValidLines(diffFiles)
	var validFindings []reviewer.Finding
	var skippedFindings []reviewer.Finding

	for _, f := range findings {
		var matched bool
		for diffPath, lines := range validLines {
			// Use suffix match to allow LLM to output just the basename
			// (e.g. "sharpTransformer.js" matching "packages/proxy/src/sharpTransformer.js")
			cleanFPath := strings.TrimPrefix(f.File, "/")
			if diffPath == f.File || strings.HasSuffix(diffPath, "/"+cleanFPath) || strings.HasSuffix(diffPath, cleanFPath) {
				if lines[f.Line] {
					f.File = diffPath // Normalize to the exact Bitbucket path
					validFindings = append(validFindings, f)
					matched = true
					break
				}
			}
		}
		if !matched {
			skippedFindings = append(skippedFindings, f)
		}
	}

	if len(skippedFindings) > 0 {
		fmt.Printf("⚠️  Skipping %d finding(s) that reference lines not in the diff:\n", len(skippedFindings))
		fmt.Println(strings.Repeat("─", 60))
		for _, f := range skippedFindings {
			fmt.Printf("   - %s:%d (%s)\n", f.File, f.Line, f.Severity)
			fmt.Printf("     %s\n\n", f.Comment)
		}
	}

	// Print findings
	fmt.Printf("\n📝 Review Findings (%d valid, %d skipped):\n", len(validFindings), len(skippedFindings))
	fmt.Println(strings.Repeat("─", 60))
	for i, f := range validFindings {
		severityEmoji := map[string]string{"error": "🔴", "warning": "🟡", "info": "🔵"}
		emoji := severityEmoji[f.Severity]
		if emoji == "" {
			emoji = "💡"
		}
		fmt.Printf("%s [%d] %s:%d\n   %s\n\n", emoji, i+1, f.File, f.Line, f.Comment)
	}

	if cfg.DryRun {
		fmt.Printf("🏃 Dry run mode (or local review) — skipping comment posting.\n")
		return nil
	}

	if len(validFindings) == 0 {
		fmt.Printf("ℹ️  No valid findings to post.\n")
		return nil
	}

	// Post comments
	fmt.Printf("💬 Posting %d inline comment(s) to PR #%s (pending=%t)...\n",
		len(validFindings), prInfo.PRNumber, cfg.Pending)

	posted := 0
	failed := 0
	for _, f := range validFindings {
		if err := bitbucket.PostInlineComment(client, prInfo, f, cfg.Pending); err != nil {
			fmt.Printf("   ❌ Failed to post comment on %s:%d: %v\n", f.File, f.Line, err)
			failed++
		} else {
			fmt.Printf("   ✅ Posted comment on %s:%d\n", f.File, f.Line)
			posted++
		}
	}

	fmt.Printf("\n📊 Summary: %d posted, %d failed, %d skipped\n", posted, failed, len(skippedFindings))

	if failed > 0 {
		return fmt.Errorf("%d comment(s) failed to post", failed)
	}
	return nil
}
