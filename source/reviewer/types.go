package reviewer

// Finding represents a single code review finding from the LLM.
type Finding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Comment  string `json:"comment"`
}
