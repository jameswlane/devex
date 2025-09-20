package progress_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/progress"
)

var _ = Describe("Progress Tracking System", func() {
	var (
		tracker *progress.Tracker
		ctx     context.Context
	)

	BeforeEach(func() {
		tracker = progress.NewTracker(nil) // No TUI program for tests
		ctx = context.Background()
	})

	Describe("Tracker", func() {
		Context("when creating operations", func() {
			It("should create a new operation with default state", func() {
				op := tracker.StartOperation("test-1", "Test Operation", "Testing", progress.OperationInstall)

				Expect(op).NotTo(BeNil())
				state := op.GetState()
				Expect(state.ID).To(Equal("test-1"))
				Expect(state.Name).To(Equal("Test Operation"))
				Expect(state.Description).To(Equal("Testing"))
				Expect(state.Type).To(Equal(progress.OperationInstall))
				Expect(state.Status).To(Equal(progress.StatusPending))
				Expect(state.Progress).To(Equal(0.0))
				Expect(state.StartTime).To(BeTemporally("~", time.Now(), time.Second))
			})

			It("should retrieve operations by ID", func() {
				op1 := tracker.StartOperation("test-1", "Test 1", "Testing", progress.OperationInstall)
				op2 := tracker.StartOperation("test-2", "Test 2", "Testing", progress.OperationCache)

				retrieved1 := tracker.GetOperation("test-1")
				retrieved2 := tracker.GetOperation("test-2")

				Expect(retrieved1).To(Equal(op1))
				Expect(retrieved2).To(Equal(op2))
			})

			It("should support child operations", func() {
				parent := tracker.StartOperation("parent", "Parent Op", "Parent", progress.OperationInstall)
				child := tracker.StartChildOperation("parent", "child", "Child Op", "Child", progress.OperationCache)

				state := child.GetState()
				Expect(state.ParentID).To(Equal("parent"))

				children := parent.GetChildren()
				Expect(children).To(HaveLen(1))
				Expect(children[0]).To(Equal(child))
			})
		})

		Context("when tracking multiple operations", func() {
			It("should return all operations", func() {
				tracker.StartOperation("op1", "Op 1", "Testing", progress.OperationInstall)
				tracker.StartOperation("op2", "Op 2", "Testing", progress.OperationCache)
				tracker.StartOperation("op3", "Op 3", "Testing", progress.OperationConfig)

				all := tracker.GetAllOperations()
				Expect(all).To(HaveLen(3))
				Expect(all).To(HaveKey("op1"))
				Expect(all).To(HaveKey("op2"))
				Expect(all).To(HaveKey("op3"))
			})
		})
	})

	Describe("Operation", func() {
		var op *progress.Operation

		BeforeEach(func() {
			op = tracker.StartOperation("test-op", "Test Operation", "Testing", progress.OperationInstall)
		})

		Context("when updating status", func() {
			It("should update status correctly", func() {
				op.SetStatus(progress.StatusRunning)
				Expect(op.GetState().Status).To(Equal(progress.StatusRunning))

				op.SetStatus(progress.StatusCompleted)
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusCompleted))
				Expect(state.EndTime).NotTo(BeNil())
				Expect(state.Progress).To(Equal(1.0))
			})

			It("should set end time for terminal states", func() {
				op.SetStatus(progress.StatusFailed)
				state := op.GetState()
				Expect(state.EndTime).NotTo(BeNil())
				Expect(*state.EndTime).To(BeTemporally("~", time.Now(), time.Second))
			})
		})

		Context("when updating progress", func() {
			It("should update progress correctly", func() {
				op.SetProgress(0.5)
				state := op.GetState()
				Expect(state.Progress).To(Equal(0.5))
				Expect(state.Status).To(Equal(progress.StatusRunning))
			})

			It("should clamp progress to valid range", func() {
				op.SetProgress(-0.5)
				Expect(op.GetState().Progress).To(Equal(0.0))

				op.SetProgress(1.5)
				Expect(op.GetState().Progress).To(Equal(1.0))
			})

			It("should change status to running when progress > 0", func() {
				Expect(op.GetState().Status).To(Equal(progress.StatusPending))

				op.SetProgress(0.1)
				Expect(op.GetState().Status).To(Equal(progress.StatusRunning))
			})
		})

		Context("when setting details", func() {
			It("should update details correctly", func() {
				op.SetDetails("Processing step 1...")
				Expect(op.GetState().Details).To(Equal("Processing step 1..."))
			})
		})

		Context("when setting errors", func() {
			It("should set error and mark as failed", func() {
				testErr := errors.New("test error")
				op.SetError(testErr)

				state := op.GetState()
				Expect(state.Error).To(Equal(testErr))
				Expect(state.Status).To(Equal(progress.StatusFailed))
				Expect(state.EndTime).NotTo(BeNil())
			})
		})

		Context("when setting metadata", func() {
			It("should store metadata correctly", func() {
				op.SetMetadata("key1", "value1")
				op.SetMetadata("key2", 42)

				state := op.GetState()
				Expect(state.Metadata["key1"]).To(Equal("value1"))
				Expect(state.Metadata["key2"]).To(Equal(42))
			})
		})

		Context("when checking completion", func() {
			It("should identify terminal states correctly", func() {
				Expect(op.IsComplete()).To(BeFalse())

				op.SetStatus(progress.StatusRunning)
				Expect(op.IsComplete()).To(BeFalse())

				op.SetStatus(progress.StatusCompleted)
				Expect(op.IsComplete()).To(BeTrue())

				op.SetStatus(progress.StatusFailed)
				Expect(op.IsComplete()).To(BeTrue())

				op.SetStatus(progress.StatusCancelled)
				Expect(op.IsComplete()).To(BeTrue())

				op.SetStatus(progress.StatusSkipped)
				Expect(op.IsComplete()).To(BeTrue())
			})
		})

		Context("when using convenience methods", func() {
			It("should complete operations correctly", func() {
				op.Complete()
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusCompleted))
				Expect(state.Progress).To(Equal(1.0))
				Expect(state.EndTime).NotTo(BeNil())
			})

			It("should fail operations correctly", func() {
				testErr := errors.New("failure")
				op.Fail(testErr)
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusFailed))
				Expect(state.Error).To(Equal(testErr))
				Expect(state.EndTime).NotTo(BeNil())
			})

			It("should skip operations correctly", func() {
				op.Skip("not needed")
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusSkipped))
				Expect(state.Details).To(Equal("not needed"))
				Expect(state.EndTime).NotTo(BeNil())
			})

			It("should cancel operations correctly", func() {
				op.Cancel()
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusCancelled))
				Expect(state.EndTime).NotTo(BeNil())
			})
		})

		Context("when measuring duration", func() {
			It("should calculate duration correctly", func() {
				start := time.Now()
				time.Sleep(10 * time.Millisecond)
				duration := op.GetDuration()
				expectedDuration := time.Since(start)
				Expect(duration).To(BeNumerically(">=", 10*time.Millisecond))
				Expect(duration).To(BeNumerically("~", expectedDuration, 50*time.Millisecond))
			})

			It("should use end time for completed operations", func() {
				time.Sleep(10 * time.Millisecond)
				op.Complete()
				duration1 := op.GetDuration()

				time.Sleep(10 * time.Millisecond)
				duration2 := op.GetDuration()

				// Duration should not change after completion
				Expect(duration2).To(Equal(duration1))
			})
		})
	})

	Describe("SteppedOperation", func() {
		var (
			op      *progress.Operation
			steps   []progress.ProgressStep
			stepper *progress.SteppedOperation
		)

		BeforeEach(func() {
			op = tracker.StartOperation("stepped", "Stepped Operation", "Multi-step", progress.OperationInstall)
			steps = []progress.ProgressStep{
				{Name: "Step 1", Description: "First step", Weight: 1.0},
				{Name: "Step 2", Description: "Second step", Weight: 2.0},
				{Name: "Step 3", Description: "Third step", Weight: 1.0},
			}
			stepper = op.NewSteppedOperation(steps)
		})

		Context("when advancing through steps", func() {
			It("should advance steps correctly", func() {
				// Initial state
				Expect(stepper.GetCurrentStep()).To(BeNil())
				Expect(op.GetState().Progress).To(Equal(0.0))

				// First step
				advanced := stepper.NextStep()
				Expect(advanced).To(BeTrue())
				currentStep := stepper.GetCurrentStep()
				Expect(currentStep.Name).To(Equal("Step 1"))
				Expect(op.GetState().Details).To(ContainSubstring("Step 1"))

				// Second step
				advanced = stepper.NextStep()
				Expect(advanced).To(BeTrue())
				currentStep = stepper.GetCurrentStep()
				Expect(currentStep.Name).To(Equal("Step 2"))

				// Third step
				advanced = stepper.NextStep()
				Expect(advanced).To(BeTrue())
				currentStep = stepper.GetCurrentStep()
				Expect(currentStep.Name).To(Equal("Step 3"))

				// No more steps
				advanced = stepper.NextStep()
				Expect(advanced).To(BeFalse())
			})

			It("should calculate progress correctly", func() {
				// Step 1 (weight 1.0 out of total 4.0)
				stepper.NextStep()
				stepper.SetStepProgress(0.5)          // 50% of step 1
				expectedProgress := (0.5 * 1.0) / 4.0 // 0.125
				Expect(op.GetState().Progress).To(BeNumerically("~", expectedProgress, 0.001))

				stepper.SetStepProgress(1.0) // 100% of step 1
				expectedProgress = 1.0 / 4.0 // 0.25
				Expect(op.GetState().Progress).To(BeNumerically("~", expectedProgress, 0.001))

				// Step 2 (weight 2.0)
				stepper.NextStep()
				stepper.SetStepProgress(0.5)             // 50% of step 2
				expectedProgress = (1.0 + 0.5*2.0) / 4.0 // 0.5
				Expect(op.GetState().Progress).To(BeNumerically("~", expectedProgress, 0.001))

				stepper.SetStepProgress(1.0)         // 100% of step 2
				expectedProgress = (1.0 + 2.0) / 4.0 // 0.75
				Expect(op.GetState().Progress).To(BeNumerically("~", expectedProgress, 0.001))

				// Step 3 (weight 1.0)
				stepper.NextStep()
				stepper.SetStepProgress(1.0)               // 100% of step 3
				expectedProgress = (1.0 + 2.0 + 1.0) / 4.0 // 1.0
				Expect(op.GetState().Progress).To(BeNumerically("~", expectedProgress, 0.001))
			})
		})
	})

	Describe("ProgressManager", func() {
		var manager *progress.ProgressManager

		BeforeEach(func() {
			manager = progress.NewProgressManager(ctx, nil)
		})

		Context("when using WithProgress", func() {
			It("should handle successful operations", func() {
				executed := false
				err := manager.WithProgress("test", "Test", "Testing", progress.OperationInstall, func(op *progress.Operation) error {
					executed = true
					op.SetProgress(0.5)
					return nil
				})

				Expect(err).NotTo(HaveOccurred())
				Expect(executed).To(BeTrue())

				op := manager.GetTracker().GetOperation("test")
				Expect(op).NotTo(BeNil())
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusCompleted))
				Expect(state.Progress).To(Equal(1.0))
			})

			It("should handle failed operations", func() {
				testErr := errors.New("test error")
				err := manager.WithProgress("test", "Test", "Testing", progress.OperationInstall, func(op *progress.Operation) error {
					return testErr
				})

				Expect(err).To(Equal(testErr))

				op := manager.GetTracker().GetOperation("test")
				Expect(op).NotTo(BeNil())
				state := op.GetState()
				Expect(state.Status).To(Equal(progress.StatusFailed))
				Expect(state.Error).To(Equal(testErr))
			})
		})
	})

	Describe("Utility Functions", func() {
		Context("when formatting durations", func() {
			It("should format seconds correctly", func() {
				Expect(progress.FormatDuration(30 * time.Second)).To(Equal("30s"))
				Expect(progress.FormatDuration(45 * time.Second)).To(Equal("45s"))
			})

			It("should format minutes correctly", func() {
				Expect(progress.FormatDuration(2 * time.Minute)).To(Equal("2.0m"))
				Expect(progress.FormatDuration(2*time.Minute + 30*time.Second)).To(Equal("2.5m"))
			})

			It("should format hours correctly", func() {
				Expect(progress.FormatDuration(2 * time.Hour)).To(Equal("2.0h"))
				Expect(progress.FormatDuration(2*time.Hour + 30*time.Minute)).To(Equal("2.5h"))
			})
		})

		Context("when getting status icons", func() {
			It("should return correct icons for all statuses", func() {
				Expect(progress.GetStatusIcon(progress.StatusPending)).To(Equal("⏳"))
				Expect(progress.GetStatusIcon(progress.StatusRunning)).To(Equal("🔄"))
				Expect(progress.GetStatusIcon(progress.StatusCompleted)).To(Equal("✅"))
				Expect(progress.GetStatusIcon(progress.StatusFailed)).To(Equal("❌"))
				Expect(progress.GetStatusIcon(progress.StatusSkipped)).To(Equal("⏭️"))
				Expect(progress.GetStatusIcon(progress.StatusCancelled)).To(Equal("🚫"))
			})
		})

		Context("when getting operation type icons", func() {
			It("should return correct icons for all operation types", func() {
				Expect(progress.GetOperationTypeIcon(progress.OperationInstall)).To(Equal("📦"))
				Expect(progress.GetOperationTypeIcon(progress.OperationCache)).To(Equal("🗄️"))
				Expect(progress.GetOperationTypeIcon(progress.OperationConfig)).To(Equal("⚙️"))
				Expect(progress.GetOperationTypeIcon(progress.OperationTemplate)).To(Equal("📋"))
				Expect(progress.GetOperationTypeIcon(progress.OperationStatus)).To(Equal("📊"))
				Expect(progress.GetOperationTypeIcon(progress.OperationSystem)).To(Equal("🖥️"))
				Expect(progress.GetOperationTypeIcon(progress.OperationBackup)).To(Equal("💾"))
				Expect(progress.GetOperationTypeIcon(progress.OperationRestore)).To(Equal("🔄"))
				Expect(progress.GetOperationTypeIcon(progress.OperationExport)).To(Equal("📤"))
				Expect(progress.GetOperationTypeIcon(progress.OperationImport)).To(Equal("📥"))
			})
		})
	})

	Describe("Thread Safety", func() {
		Context("when accessing operations concurrently", func() {
			It("should handle concurrent operations safely", func() {
				const numGoroutines = 10
				const operationsPerGoroutine = 5

				var wg sync.WaitGroup
				wg.Add(numGoroutines)

				for i := 0; i < numGoroutines; i++ {
					go func(workerID int) {
						defer wg.Done()
						for j := 0; j < operationsPerGoroutine; j++ {
							opID := fmt.Sprintf("worker-%d-op-%d", workerID, j)
							op := tracker.StartOperation(opID, "Concurrent Op", "Testing", progress.OperationInstall)

							// Simulate some work
							op.SetProgress(0.5)
							op.SetDetails("Working...")
							time.Sleep(time.Millisecond)
							op.Complete()
						}
					}(i)
				}

				wg.Wait()

				// Verify all operations were created
				all := tracker.GetAllOperations()
				Expect(all).To(HaveLen(numGoroutines * operationsPerGoroutine))

				// Verify all operations completed successfully
				for _, op := range all {
					state := op.GetState()
					Expect(state.Status).To(Equal(progress.StatusCompleted))
					Expect(state.Progress).To(Equal(1.0))
				}
			})
		})

		Context("when updating operation state concurrently", func() {
			It("should handle concurrent updates safely", func() {
				op := tracker.StartOperation("concurrent-test", "Concurrent Test", "Testing", progress.OperationInstall)

				const numGoroutines = 20
				var wg sync.WaitGroup
				wg.Add(numGoroutines)

				for i := 0; i < numGoroutines; i++ {
					go func(workerID int) {
						defer wg.Done()
						// Each goroutine performs different operations
						switch workerID % 4 {
						case 0:
							op.SetProgress(float64(workerID) / 100.0)
						case 1:
							op.SetDetails(fmt.Sprintf("Worker %d", workerID))
						case 2:
							op.SetMetadata(fmt.Sprintf("key-%d", workerID), workerID)
						case 3:
							// Just read the state
							_ = op.GetState()
						}
					}(i)
				}

				wg.Wait()

				// Operation should still be in a valid state
				state := op.GetState()
				Expect(state).NotTo(BeNil())
				Expect(state.Status).To(BeElementOf(progress.StatusPending, progress.StatusRunning))
			})
		})
	})

	Describe("ProgressListener", func() {
		var (
			listener *mockProgressListener
			received []*progress.ProgressState
			mutex    sync.Mutex
		)

		BeforeEach(func() {
			received = make([]*progress.ProgressState, 0)
			listener = &mockProgressListener{
				callback: func(state *progress.ProgressState) {
					mutex.Lock()
					defer mutex.Unlock()
					received = append(received, state)
				},
			}
			tracker.AddListener(listener)
		})

		Context("when operations are updated", func() {
			It("should notify listeners of progress updates", func() {
				// Track all status changes for this specific operation
				var allStatuses []progress.Status
				var statusMutex sync.Mutex

				// Create a custom listener that tracks all status changes
				customListener := &mockProgressListener{
					callback: func(state *progress.ProgressState) {
						if state.ID == "listener-test" {
							statusMutex.Lock()
							allStatuses = append(allStatuses, state.Status)
							statusMutex.Unlock()

							// Also add to general received for compatibility
							mutex.Lock()
							received = append(received, state)
							mutex.Unlock()
						}
					},
				}
				tracker.AddListener(customListener)

				// Start operation
				op := tracker.StartOperation("listener-test", "Listener Test", "Testing", progress.OperationInstall)

				// Wait for initial pending notification
				Eventually(func() int {
					statusMutex.Lock()
					defer statusMutex.Unlock()
					return len(allStatuses)
				}).Should(BeNumerically(">=", 1))

				// Verify initial status is pending
				statusMutex.Lock()
				Expect(allStatuses[0]).To(Equal(progress.StatusPending))
				statusMutex.Unlock()

				// Update progress (should trigger running status)
				op.SetProgress(0.5)

				// Wait for running status
				Eventually(func() bool {
					statusMutex.Lock()
					defer statusMutex.Unlock()
					for _, status := range allStatuses {
						if status == progress.StatusRunning {
							return true
						}
					}
					return false
				}, "1s", "10ms").Should(BeTrue(), "Should receive running status notification")

				// Complete the operation
				op.Complete()

				// Wait for completed status
				Eventually(func() bool {
					statusMutex.Lock()
					defer statusMutex.Unlock()
					for _, status := range allStatuses {
						if status == progress.StatusCompleted {
							return true
						}
					}
					return false
				}, "1s", "10ms").Should(BeTrue(), "Should receive completed status notification")

				// Final verification
				statusMutex.Lock()
				defer statusMutex.Unlock()

				Expect(allStatuses).To(ContainElement(progress.StatusPending))
				Expect(allStatuses).To(ContainElement(progress.StatusRunning))
				Expect(allStatuses).To(ContainElement(progress.StatusCompleted))
			})
		})
	})
})

// mockProgressListener implements ProgressListener for testing
type mockProgressListener struct {
	callback func(*progress.ProgressState)
}

func (m *mockProgressListener) OnProgressUpdate(state *progress.ProgressState) {
	if m.callback != nil {
		m.callback(state)
	}
}
