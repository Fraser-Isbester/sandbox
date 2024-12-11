# git-diff-review

This builds a git extension that does code review.

## todo

- [x] git diff parser
- [x] stdout review (dev context)
- [ ] embed the repo
- [ ] github pull request review (gha context)

## Usage

```shell
➜  git-diff-review git:(git-diff-review) ✗ make build
go build -o ~/bin/git-diff-review ./cmd/git-diff-review
➜  git-diff-review git:(git-diff-review) ✗ git diff-review
cmd/git-diff-review/main.go:0: main.go:15: Consider using `context.TODO()` instead of `context.Background()` if the context is not actively used or propagated.
cmd/git-diff-review/main.go:30: Error handling could be improved by using a multi-line error check pattern for better readability.
cmd/git-diff-review/main.go:35: Consider adding a check for empty `reviews` slice before the loop to avoid unnecessary iteration.
cmd/git-diff-review/main.go:36: Use `log.Printf` instead of `fmt.Printf` for consistency with error logging elsewhere in the file.
internal/git/diff.go:0: diff.go:31: Consider using a pointer for the `repo` field to avoid unnecessary copying of potentially large struct.
internal/git/diff.go:61-65: Replace exec.Command with go-git's native diff functionality for better performance and maintainability.
internal/git/diff.go:73: Use a more efficient string splitting method, like bufio.Scanner, to handle large diffs without loading the entire output into memory.
internal/reviewer/reviewer.go:0: Consider adding a comment explaining the purpose of the `llmClient` field in the `Reviewer` struct.
internal/reviewer/reviewer.go:31: Add error handling for the `regexp.MatchString` function to avoid potential panics.
internal/reviewer/reviewer.go:37: Consider making `skipPatterns` a constant or package-level variable to avoid recreating it on every function call.
internal/reviewer/reviewer.go:48: Add a check for empty `diffs` slice to avoid unnecessary processing.
internal/reviewer/reviewer.go:57: Consider adding a severity level to the LLM response and use it in the `Review` struct instead of hardcoding "info".
```