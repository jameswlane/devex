package mocks_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("MockUtils", func() {
	var mockUtils *mocks.MockUtils

	BeforeEach(func() {
		mockUtils = mocks.NewMockUtils()
	})

	Describe("RunShellCommand", func() {
		It("should execute a shell command successfully", func() {
			output, err := mockUtils.RunShellCommand("test-command")
			Expect(err).ToNot(HaveOccurred())
			Expect(output).To(Equal("mock shell output"))
			Expect(mockUtils.Commands).To(ContainElement("test-command"))
		})

		It("should return an error for a failing command", func() {
			mockUtils.FailCommand("fail-command")
			_, err := mockUtils.RunShellCommand("fail-command")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("mock RunShellCommand failed"))
		})
	})

	Describe("EnsureDir", func() {
		It("should ensure a directory successfully", func() {
			err := mockUtils.EnsureDir("/valid/path")
			Expect(err).ToNot(HaveOccurred())
			Expect(mockUtils.EnsureDirCalled).To(BeTrue())
		})

		It("should return an error for a failing directory creation", func() {
			mockUtils.FailCommand("/invalid/path")
			err := mockUtils.EnsureDir("/invalid/path")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("mock EnsureDir failed"))
		})
	})

	Describe("DownloadFileWithContext", func() {
		It("should simulate a file download", func() {
			ctx := context.Background()
			err := mockUtils.DownloadFileWithContext(ctx, "http://example.com/file", "/tmp/file")
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error for a failing download", func() {
			mockUtils.FailCommand("http://example.com/file")
			ctx := context.Background()
			err := mockUtils.DownloadFileWithContext(ctx, "http://example.com/file", "/tmp/file")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("mock DownloadFileWithContext failed"))
		})
	})
})
