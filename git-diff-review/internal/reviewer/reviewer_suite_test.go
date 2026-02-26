package reviewer_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestReviewer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reviewer Suite")
}
