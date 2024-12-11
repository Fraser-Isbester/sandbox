package main

import (
	"fmt"
	"log"
	"os"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
)

func main() {
	// Get current working directory as repo path
	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Initialize diff provider
	diffProvider := git.NewDiffProvider(repoPath)

	// Get current diff
	diff, err := diffProvider.GetCurrentDiff()
	if err != nil {
		log.Fatalf("Failed to get diff: %v", err)
	}

	// Initialize reviewer
	reviewer := reviewer.NewReviewer()

	// Get review comments
	reviews, err := reviewer.ReviewDiff(diff)
	if err != nil {
		log.Fatalf("Failed to review diff: %v", err)
	}

	// Print reviews to stdout
	for _, review := range reviews {
		fmt.Printf("%s:%d: %s [%s]\n",
			review.FilePath,
			review.LineNumber,
			review.Suggestion,
			review.Severity,
		)
	}
}
