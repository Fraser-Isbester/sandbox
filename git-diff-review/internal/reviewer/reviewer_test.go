package reviewer_test

import (
	"context"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reviewer", func() {
	var (
		r   *reviewer.Reviewer
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		r, err = reviewer.NewReviewer()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("ReviewDiffs", func() {
		It("should filter out go.mod files", func() {
			diffs := []git.DiffEntry{
				{Path: "go.mod", Content: "module test"},
				{Path: "main.go", Content: "func main() {}"},
			}

			reviews, err := r.ReviewDiffs(ctx, diffs)
			Expect(err).NotTo(HaveOccurred())
			Expect(reviews).To(HaveLen(1))
		})

		It("should return structured reviews", func() {
			diffs := []git.DiffEntry{{
				Path: "main.go",
				Content: `func main() {
                   var x int
               }`,
			}}

			reviews, err := r.ReviewDiffs(ctx, diffs)
			Expect(err).NotTo(HaveOccurred())
			Expect(reviews[0].Comments).To(ContainElement(HaveField("Type", "issue")))
			Expect(reviews[0].Summary).NotTo(BeEmpty())
		})

		It("skips LLM call when no diffs", func() {
			diffs := []git.DiffEntry{}

			reviews, err := r.ReviewDiffs(ctx, diffs)
			Expect(err).NotTo(HaveOccurred())
			Expect(reviews).To(BeEmpty())
		})
	})
})
