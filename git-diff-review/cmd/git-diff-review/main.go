package main

import (
	"context"
	"flag"
	"os"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/format"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/log"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
)

func main() {
	ctx := context.Background()
	log.Logger.Info().Msg("Starting git-diff-review")

	outputFormat := flag.String("format", "json", "output format (json, pretty)")
	flag.Parse()

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

	reviewer, err := reviewer.NewReviewer(reviewer.Config{Provider: reviewer.OpenAIProvider})
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to initialize reviewer")
	}

	reviews, err := reviewer.ReviewDiffs(ctx, diffs)
	if err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to review diffs")
	}

	formatter := createFormatter(*outputFormat)
	if err := formatter.Format(reviews, os.Stdout); err != nil {
		log.Logger.Fatal().Err(err).Msg("Failed to format output")
	}
}

func createFormatter(formatFlag string) format.Formatter {
	switch formatFlag {
	case "pretty":
		return format.NewPrettyFormatter()
	case "json":
		return format.NewJSONFormatter()
	default:
		log.Logger.Fatal().Msgf("Unsupported format: %s", formatFlag)
		return nil
	}
}
