package git_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
)

var _ = Describe("DiffProvider", func() {
	var (
		tmpDir string
		dp     *git.DiffProvider
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "git-test")
		Expect(err).NotTo(HaveOccurred())

		setupTestRepo(tmpDir)
		dp, err = git.NewDiffProvider(tmpDir)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	It("should detect modifications", func() {
		createTestFile(tmpDir, "test.txt", "content")

		diffs, err := dp.GetCurrentDiff()
		Expect(err).NotTo(HaveOccurred())
		Expect(diffs).To(HaveLen(1))
		Expect(diffs[0].Type).To(Equal(git.Add))
	})
})

func setupTestRepo(path string) {
	cmd := exec.Command("git", "init", path)
	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	cmd = exec.Command("git", "-C", path, "config", "user.email", "test@example.com")
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())

	cmd = exec.Command("git", "-C", path, "config", "user.name", "Test User")
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())
}

func createTestFile(dir, name, content string) {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	Expect(err).NotTo(HaveOccurred())

	cmd := exec.Command("git", "-C", dir, "add", name)
	err = cmd.Run()
	Expect(err).NotTo(HaveOccurred())
}
