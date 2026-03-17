package bitbucket

// PRInfo holds the parsed components of a Bitbucket PR URL.
type PRInfo struct {
	Workspace string
	RepoSlug  string
	PRNumber  string
	BaseURL   string // e.g. "https://api.bitbucket.org/2.0" for Cloud
}

// PRMetadata holds metadata about a pull request fetched from the API.
type PRMetadata struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Source struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	} `json:"source"`
	Destination struct {
		Branch struct {
			Name string `json:"name"`
		} `json:"branch"`
	} `json:"destination"`
}

// InlineCommentRequest is the payload for creating an inline comment on Bitbucket Cloud.
type InlineCommentRequest struct {
	Content InlineCommentContent `json:"content"`
	Inline  InlinePosition       `json:"inline"`
	Pending bool                 `json:"pending"`
}

// InlineCommentContent represents the comment body.
type InlineCommentContent struct {
	Raw string `json:"raw"`
}

// InlinePosition specifies where the inline comment is anchored.
type InlinePosition struct {
	Path string `json:"path"`
	To   int    `json:"to,omitempty"`
	From int    `json:"from,omitempty"`
}
