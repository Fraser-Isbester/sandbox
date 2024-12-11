package reviewer

// Review represents a code review result
type Review struct {
	FilePath   string
	LineNumber int
	Suggestion string
	Severity   string
}

// Reviewer handles code review logic
type Reviewer struct{}

// NewReviewer creates a new Reviewer instance
func NewReviewer() *Reviewer {
	return &Reviewer{}
}

// ReviewDiff analyzes the provided diff and returns review comments
func (r *Reviewer) ReviewDiff(diff []byte) ([]Review, error) {
	// For v1, we'll just return a simple placeholder review
	return []Review{
		{
			FilePath:   "example.go",
			LineNumber: 1,
			Suggestion: "Placeholder review comment",
			Severity:   "info",
		},
	}, nil
}
