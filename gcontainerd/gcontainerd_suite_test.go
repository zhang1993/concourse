package gcontainerd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGcontainerd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gcontainerd Suite")
}
