package bytefmt_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestFormatters(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bytefmt Suite")
}
