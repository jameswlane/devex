package mocks_test

import (
	"context"
	"fmt"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Concurrent mock command execution", func() {
	It("records every command and installation when callers run concurrently", func() {
		executor := mocks.NewMockCommandExecutor()
		var callers sync.WaitGroup
		failures := make(chan error, 128)
		start := make(chan struct{})
		expected := make([]string, 0, 128)
		for i := range 64 {
			name := fmt.Sprintf("package-%d", i)
			install := "sudo apt-get install -y " + name
			expected = append(expected, install, "echo "+name)
			callers.Add(1)
			go func() {
				defer callers.Done()
				<-start
				if _, err := executor.RunShellCommand(install); err != nil {
					failures <- err
				}
				if _, err := executor.RunCommand(context.Background(), "echo", name); err != nil {
					failures <- err
				}
			}()
		}
		close(start)
		callers.Wait()
		close(failures)
		for err := range failures {
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(executor.Commands).To(ConsistOf(expected))
		Expect(executor.InstallationState).To(HaveLen(64))
		for _, installed := range executor.InstallationState {
			Expect(installed).To(BeTrue())
		}
	})
})
