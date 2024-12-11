package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
)

func main() {
	ctx := context.Background()

	repoPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	diffProvider, err := git.NewDiffProvider(repoPath)
	if err != nil {
		log.Fatalf("Failed to initialize diff provider: %v", err)
	}

	diffs, err := diffProvider.GetCurrentDiff()
	if err != nil {
		log.Fatalf("Failed to get diff: %v", err)
	}

	reviewer, err := reviewer.NewReviewer()
	if err != nil {
		log.Fatalf("Failed to initialize reviewer: %v", err)
	}

	reviews, err := reviewer.ReviewDiffs(ctx, diffs)
	if err != nil {
		log.Fatalf("Failed to review diffs: %v", err)
	}

	for _, review := range reviews {
		fmt.Printf("%s:%d: %s\n\n", review.FilePath, review.LineNumber, review.Suggestion)
	}
}
