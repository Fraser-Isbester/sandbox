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
➜  git-diff-review git:(git-diff-review) ✗ git diff-review | jq
[
  {
    "file": "git-diff-review/Makefile",
    "comments": [
      {
        "type": "praise",
        "severity": "info",
        "line": 5,
        "message": "Good practice to include a test target in the Makefile."
      },
      {
        "type": "suggestion",
        "severity": "info",
        "line": 9,
        "message": "Consider adding a .PHONY directive for non-file targets.",
        "suggestion": "Add '.PHONY: build test lint' at the beginning of the Makefile."
      },
      ...
  {
    "file": "git-diff-review/internal/git/diff.go",
    "comments": [
      {
        "type": "issue",
        "severity": "warning",
        "line": 1,
        "message": "The determineChangeType function is being removed without a clear replacement. This may lead to functionality loss or errors elsewhere in the codebase.",
        "suggestion": "Consider keeping the function or ensuring its functionality is handled elsewhere before removal."
      },
      ...
```
