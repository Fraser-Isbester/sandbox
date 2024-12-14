package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/log"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
)

func main() {
	ctx := context.Background()

	log.WriterInstance.StartSpinner()
	defer log.WriterInstance.StopSpinner()

	log.Logger.Info().Msg("Starting git-diff-review")

	repoPath, err := os.Getwd()
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to get working directory")
	}

	diffProvider, err := git.NewDiffProvider(repoPath)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to initialize diff provider")
	}

	diffs, err := diffProvider.GetCurrentDiff()
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to get diff")
	}

	reviewer, err := reviewer.NewReviewer(reviewer.Config{Provider: reviewer.AnthropicProvider})
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to initialize reviewer")
	}

	reviews, err := reviewer.ReviewDiffs(ctx, diffs)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to review diffs")
	}

	output, err := json.Marshal(reviews)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to marshal reviews")
	}
	fmt.Println(string(output))
}
