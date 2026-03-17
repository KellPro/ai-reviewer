package bitbucket

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// ParsePRURL parses a Bitbucket Cloud PR URL into its components.
// Supported formats:
//   - https://bitbucket.org/{workspace}/{repo}/pull-requests/{id}
//   - https://bitbucket.org/{workspace}/{repo}/pull-requests/{id}/...
func ParsePRURL(rawURL string) (*PRInfo, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Match Bitbucket Cloud URLs
	re := regexp.MustCompile(`^/([^/]+)/([^/]+)/pull-requests/(\d+)`)
	matches := re.FindStringSubmatch(u.Path)
	if matches == nil {
		return nil, fmt.Errorf("URL does not match Bitbucket PR format: %s\nExpected: https://bitbucket.org/{workspace}/{repo}/pull-requests/{id}", rawURL)
	}

	return &PRInfo{
		Workspace: matches[1],
		RepoSlug:  matches[2],
		PRNumber:  matches[3],
		BaseURL:   "https://api.bitbucket.org/2.0",
	}, nil
}

// GetPRMetadata fetches metadata about a pull request (title, source branch, etc.).
func GetPRMetadata(client *Client, pr *PRInfo) (*PRMetadata, error) {
	apiURL := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%s",
		pr.BaseURL, pr.Workspace, pr.RepoSlug, pr.PRNumber)

	body, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetching PR metadata: %w", err)
	}

	var meta PRMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parsing PR metadata: %w", err)
	}
	return &meta, nil
}

// GetDiff fetches the unified diff for a pull request.
func GetDiff(client *Client, pr *PRInfo) (string, error) {
	apiURL := fmt.Sprintf("%s/repositories/%s/%s/pullrequests/%s/diff",
		pr.BaseURL, pr.Workspace, pr.RepoSlug, pr.PRNumber)

	body, err := client.GetRaw(apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching diff: %w", err)
	}

	return string(body), nil
}

// GetFileContent fetches the content of a file from the repo at a specific ref (branch/commit).
// Returns empty string and no error if the file doesn't exist (404).
func GetFileContent(client *Client, baseURL, repo, ref, filePath string) (string, error) {
	// Bitbucket Cloud src endpoint: /2.0/repositories/{workspace}/{repo}/src/{ref}/{path}
	apiURL := fmt.Sprintf("%s/repositories/%s/src/%s/%s",
		baseURL, repo,
		url.PathEscape(ref), filePath)

	body, err := client.GetRaw(apiURL)
	if err != nil {
		// 404 means the file doesn't exist, which is fine
		if strings.Contains(err.Error(), "HTTP 404") {
			return "", nil
		}
		return "", fmt.Errorf("fetching %s: %w", filePath, err)
	}
	return string(body), nil
}
