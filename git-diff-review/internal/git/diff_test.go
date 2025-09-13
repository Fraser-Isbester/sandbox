package git_test

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Git", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "git-test")
		Expect(err).NotTo(HaveOccurred())

		setupGitRepo(tempDir)
	})

	AfterEach(func() {
		os.RemoveAll(tempDir)
	})

	Describe("NewDiffProvider", func() {
		It("fails when not in git repo", func() {
			emptyDir, _ := os.MkdirTemp("", "empty")
			defer os.RemoveAll(emptyDir)

			_, err := git.NewDiffProvider(emptyDir)
			Expect(err).To(MatchError(ContainSubstring("not in a git repository")))
		})

		It("succeeds in valid git repo", func() {
			provider, err := git.NewDiffProvider(tempDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(provider).NotTo(BeNil())
		})

		It("handles no changes gracefully", func() {
			provider, _ := git.NewDiffProvider(tempDir)

			diffs, err := provider.GetCurrentDiff()
			Expect(err).NotTo(HaveOccurred())
			Expect(diffs).To(BeEmpty())
		})
	})

	Describe("GetCurrentDiff", func() {
		It("detects modified files", func() {
			provider, _ := git.NewDiffProvider(tempDir)

			writeFile(tempDir, "test.go", "modified content")

			diffs, err := provider.GetCurrentDiff()
			Expect(err).NotTo(HaveOccurred())
			Expect(diffs).To(HaveLen(1))
			Expect(diffs[0].Path).To(Equal("test.go"))
			Expect(diffs[0].Type).To(Equal(git.Modify))
		})
	})
})

func setupGitRepo(dir string) {
	runCmd(dir, "git", "init")
	runCmd(dir, "git", "config", "user.email", "test@example.com")
	runCmd(dir, "git", "config", "user.name", "Test User")
	writeFile(dir, "test.go", "initial content")
	runCmd(dir, "git", "add", ".")
	runCmd(dir, "git", "commit", "-m", "initial commit")
}

func writeFile(dir, name, content string) {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	Expect(err).NotTo(HaveOccurred())
}

func runCmd(dir string, command ...string) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = dir
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())
}
