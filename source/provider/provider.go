package provider

import (
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	"github.com/KellPro/ai-reviewer/source/bitbucket"
)

// ReviewContext abstracts the source of the diff and files (e.g., Bitbucket API vs local Git).
type ReviewContext interface {
	GetDiff() (string, error)
	ReadRepoFile(filePath string) (string, error)
	SearchRepoCode(query string) (string, error)
}

// bitbucketContext implements ReviewContext using the Bitbucket API.
type bitbucketContext struct {
	client       *bitbucket.Client
	prInfo       *bitbucket.PRInfo
	commitRef    string
	repoFullName string // e.g., "workspace/repoSlug"
}

// NewBitbucketContext creates a new ReviewContext for fetching via Bitbucket API.
func NewBitbucketContext(client *bitbucket.Client, prInfo *bitbucket.PRInfo, commitRef, repoFullName string) ReviewContext {
	return &bitbucketContext{
		client:       client,
		prInfo:       prInfo,
		commitRef:    commitRef,
		repoFullName: repoFullName,
	}
}

func (b *bitbucketContext) GetDiff() (string, error) {
	return bitbucket.GetDiff(b.client, b.prInfo)
}

func (b *bitbucketContext) ReadRepoFile(filePath string) (string, error) {
	return bitbucket.GetFileContent(b.client, b.prInfo.BaseURL, b.repoFullName, b.commitRef, filePath)
}

func (b *bitbucketContext) SearchRepoCode(query string) (string, error) {
	searchURL := fmt.Sprintf("%s/workspaces/%s/search/code?search_query=%s", b.prInfo.BaseURL, b.prInfo.Workspace, url.QueryEscape(query))
	body, err := b.client.GetRaw(searchURL)
	if err != nil {
		// Log but don't fail, bitbucket search might not be enabled or user might not have access
		return "", fmt.Errorf("bitbucket code search failed: %w", err)
	}
	return string(body), nil
}

// gitPRContext implements ReviewContext using a local Git repository for a PR.
type gitPRContext struct {
	path       string
	baseCommit string // Destination branch commit or name
	headCommit string // Source branch commit or name
}

// NewGitPRContext creates a ReviewContext for a local git repo comparing two branches/commits.
func NewGitPRContext(path string, baseCommit, headCommit string) ReviewContext {
	return &gitPRContext{
		path:       path,
		baseCommit: baseCommit,
		headCommit: headCommit,
	}
}

func (g *gitPRContext) GetDiff() (string, error) {
	// e.g. git diff base...head
	cmd := exec.Command("git", "diff", fmt.Sprintf("%s...%s", g.baseCommit, g.headCommit))
	cmd.Dir = g.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %v\nOutput: %s", err, string(out))
	}
	return string(out), nil
}

func (g *gitPRContext) ReadRepoFile(fileName string) (string, error) {
	refPath := fmt.Sprintf("%s:%s", g.headCommit, fileName)
	cmd := exec.Command("git", "show", refPath)
	cmd.Dir = g.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If the file doesn't exist, git show will exit with an error. We return empty string.
		outStr := string(out)
		if strings.Contains(outStr, "exists on disk, but not in") || strings.Contains(outStr, "does not exist in") || strings.Contains(outStr, "fatal: path") || strings.Contains(outStr, "Not a valid object name") {
			return "", nil
		}
		return "", fmt.Errorf("git show failed on %s: %v\nOutput: %s", refPath, err, outStr)
	}
	return string(out), nil
}

func (g *gitPRContext) SearchRepoCode(query string) (string, error) {
	cmd := exec.Command("git", "grep", "-n", "-I", query, g.headCommit)
	cmd.Dir = g.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "fatal") || strings.Contains(outStr, "error") {
			return "", fmt.Errorf("git grep failed: %v\nOutput: %s", err, outStr)
		}
		return "", nil // Exit code 1 means no matches
	}
	return string(out), nil
}

// gitStagedContext implements ReviewContext using local staged changes.
type gitStagedContext struct {
	path string
}

// NewGitStagedContext creates a ReviewContext exploring local staged changes.
func NewGitStagedContext(path string) ReviewContext {
	return &gitStagedContext{path: path}
}

func (g *gitStagedContext) GetDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	cmd.Dir = g.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff --cached failed: %v\nOutput: %s", err, string(out))
	}
	return string(out), nil
}

func (g *gitStagedContext) ReadRepoFile(fileName string) (string, error) {
	// For staged files, we can just view what's staged in the index index: git show :<file>
	refPath := fmt.Sprintf(":%s", fileName)
	cmd := exec.Command("git", "show", refPath)
	cmd.Dir = g.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "not in the cache") || strings.Contains(outStr, "fatal: path") {
			return "", nil
		}
		return "", fmt.Errorf("git show %s failed: %v\nOutput: %s", refPath, err, outStr)
	}
	return string(out), nil
}

func (g *gitStagedContext) SearchRepoCode(query string) (string, error) {
	cmd := exec.Command("git", "grep", "-n", "-I", query)
	cmd.Dir = g.path
	out, err := cmd.CombinedOutput()
	if err != nil {
		outStr := string(out)
		if strings.Contains(outStr, "fatal") || strings.Contains(outStr, "error") {
			return "", fmt.Errorf("git grep failed: %v\nOutput: %s", err, outStr)
		}
		return "", nil
	}
	return string(out), nil
}

// IsGitRepo checks if the given path is a valid git repository.
func IsGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = path
	err := cmd.Run()
	return err == nil
}

// GetRemoteName checks if the local git remotes contain the expected workspace and repo slug and returns the remote name.
func GetRemoteName(path, workspace, repoSlug string) (string, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to list remotes: %w", err)
	}
	
	expected := fmt.Sprintf("%s/%s", workspace, repoSlug)
	re := regexp.MustCompile(`(?m)^([^\s]+)\s+.*` + regexp.QuoteMeta(expected))
	matches := re.FindStringSubmatch(string(out))
	if matches != nil {
		return matches[1], nil
	}
	return "", fmt.Errorf("no remote found matching %s", expected)
}

// GetBitbucketRepoFromPath parses the local git remotes to find a Bitbucket repository and extracts the workspace and repo slug.
func GetBitbucketRepoFromPath(path string) (string, string, error) {
	cmd := exec.Command("git", "remote", "-v")
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("failed to get git remotes: %v", err)
	}

	// Examples:
	// origin  git@bitbucket.org:workspace/repo.git (fetch)
	// origin  https://bitbucket.org/workspace/repo.git (fetch)
	re := regexp.MustCompile(`(?m)bitbucket\.org[:/]([a-zA-Z0-9._-]+)/([a-zA-Z0-9._-]+)(?:\.git)?`)
	matches := re.FindStringSubmatch(string(out))
	if matches == nil {
		return "", "", fmt.Errorf("no bitbucket remote found")
	}

	return matches[1], strings.TrimSuffix(matches[2], ".git"), nil
}

// SwitchAndPull attempts to switch to the target branch and pull with --rebase from the remote.
func SwitchAndPull(path, branch, remote string) error {
	cmd := exec.Command("git", "switch", branch)
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git switch %s failed: %v\nOutput: %s", branch, err, string(out))
	}

	cmd = exec.Command("git", "pull", "--rebase", remote, branch)
	cmd.Dir = path
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git pull --rebase %s %s failed: %v\nOutput: %s", remote, branch, err, string(out))
	}
	return nil
}
