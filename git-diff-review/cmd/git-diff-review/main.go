package main

import (
	"log"
	"os"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
)

func main() {
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

	for _, diff := range diffs {
		log.Printf("Change in %s: %v", diff.Path, diff.Type)
	}
}
