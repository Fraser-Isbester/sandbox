package format

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
)

type Formatter interface {
	Format(reviews []reviewer.ReviewResponse, writer io.Writer) error
}

type PrettyFormatter struct {
	fileColor *color.Color
	typeColor *color.Color
}

func newPrettyFormatter() *PrettyFormatter {
	return &PrettyFormatter{
		fileColor: color.New(color.FgCyan),
		typeColor: color.New(color.FgGreen),
	}
}

func (p *PrettyFormatter) Format(reviews []reviewer.ReviewResponse, writer io.Writer) error {
	const (
		indent    = "    "
		codeBlock = "```"
	)

	for _, review := range reviews {
		fmt.Fprintf(writer, "%s\n", p.fileColor.Sprint(review.File))

		for _, comment := range review.Comments {
			fmt.Fprintf(writer, "%s%d %s: %s\n",
				indent,
				comment.Line,
				p.typeColor.Sprintf("[%s]", comment.Type),
				comment.Message,
			)

			if comment.Suggestion != "" {
				lang := strings.TrimPrefix(filepath.Ext(review.File), ".")
				fmt.Fprintf(writer, "%s%s%s\n", indent, codeBlock, lang)
				fmt.Fprintf(writer, "%s%s\n", indent, comment.Suggestion)
				fmt.Fprintf(writer, "%s%s\n", indent, codeBlock)
			}
		}
	}
	return nil
}

type JSONFormatter struct{}

func newJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

func (j *JSONFormatter) Format(reviews []reviewer.ReviewResponse, writer io.Writer) error {
	output, err := json.MarshalIndent(reviews, "", "    ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(writer, string(output))
	return err
}

func NewFormatter(formatFlag string) Formatter {
	switch formatFlag {
	case "pretty":
		return newPrettyFormatter()
	case "json":
		return newJSONFormatter()
	case "github":
		return newGitHubFormatter()
	default:
		// log.Logger().Fatal().Msgf("Unsupported format: %s", formatFlag)
		return nil
	}
}
