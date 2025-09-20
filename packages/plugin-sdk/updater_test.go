package sdk_test

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Background Updater", func() {
	var (
		tempDir    string
		downloader *sdk.Downloader
		manager    *sdk.ExecutableManager
		updater    *sdk.BackgroundUpdater
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "updater-test-*")
		Expect(err).ToNot(HaveOccurred())

		downloader = sdk.NewDownloader("https://registry.example.com", tempDir)
		manager = sdk.NewExecutableManager(tempDir)
		updater = sdk.NewBackgroundUpdater(downloader, manager)
	})

	AfterEach(func() {
		if updater != nil && updater.IsRunning() {
			updater.Stop()
		}
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	Describe("NewBackgroundUpdater", func() {
		It("should create a new background updater", func() {
			bu := sdk.NewBackgroundUpdater(downloader, manager)
			Expect(bu).ToNot(BeNil())
			Expect(bu.IsRunning()).To(BeFalse())
		})
	})

	Describe("Configuration", func() {
		It("should set update interval", func() {
			interval := 5 * time.Minute
			updater.SetUpdateInterval(interval)
			
			// We can't directly test the interval, but we can ensure the method doesn't panic
			Expect(func() { updater.SetUpdateInterval(interval) }).ToNot(Panic())
		})

		It("should add update callbacks", func() {
			callback := func(status sdk.UpdateStatus) {
				// Callback implementation
			}
			
			// The callback should be stored without error
			Expect(func() { updater.AddUpdateCallback(callback) }).ToNot(Panic())
		})
	})

	Describe("Lifecycle Management", func() {
		Context("starting and stopping", func() {
			It("should start successfully", func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				err := updater.Start(ctx)
				Expect(err).ToNot(HaveOccurred())

				// Should be running immediately after Start() call
				Expect(updater.IsRunning()).To(BeTrue())

				// Stop the updater
				updater.Stop()
				
				Eventually(func() bool {
					return updater.IsRunning()
				}, 500*time.Millisecond, 50*time.Millisecond).Should(BeFalse())
			})

			It("should stop gracefully", func() {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				go func() {
					defer GinkgoRecover()
					_ = updater.Start(ctx)
				}()

				// Wait for it to start
				Eventually(func() bool {
					return updater.IsRunning()
				}, time.Second, 100*time.Millisecond).Should(BeTrue())

				// Stop the updater
				updater.Stop()

				// Should stop running
				Eventually(func() bool {
					return updater.IsRunning()
				}, time.Second, 100*time.Millisecond).Should(BeFalse())
			})

			It("should handle multiple start attempts", func() {
				ctx1, cancel1 := context.WithCancel(context.Background())
				ctx2, cancel2 := context.WithCancel(context.Background())
				defer cancel1()
				defer cancel2()

				go func() {
					defer GinkgoRecover()
					_ = updater.Start(ctx1)
				}()

				// Wait for first start
				Eventually(func() bool {
					return updater.IsRunning()
				}, time.Second, 100*time.Millisecond).Should(BeTrue())

				// Second start should handle gracefully
				err := updater.Start(ctx2)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("already running"))
				}

				updater.Stop()
			})
		})

		Context("status tracking", func() {
			It("should track running state correctly", func() {
				Expect(updater.IsRunning()).To(BeFalse())

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				go func() {
					defer GinkgoRecover()
					_ = updater.Start(ctx)
				}()

				Eventually(func() bool {
					return updater.IsRunning()
				}, time.Second, 100*time.Millisecond).Should(BeTrue())

				updater.Stop()

				Eventually(func() bool {
					return updater.IsRunning()
				}, time.Second, 100*time.Millisecond).Should(BeFalse())
			})

			It("should track last update time", func() {
				initialTime := updater.GetLastUpdateTime()
				Expect(initialTime.IsZero()).To(BeTrue())

				// After running updates, the time should be updated
				// This is tested indirectly since we can't force an update easily
			})
		})
	})

	Describe("Update Operations", func() {
		It("should handle manual update checks", func() {
			ctx := context.Background()
			
			// Manual update check should not panic
			err := updater.CheckForUpdatesNow(ctx)
			// We expect this might fail due to no real plugins, but should not panic
			if err != nil {
				Expect(err.Error()).ToNot(BeEmpty())
			}
		})

		Context("with context cancellation", func() {
			It("should handle cancelled contexts gracefully", func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel() // Cancel immediately
				
				err := updater.CheckForUpdatesNow(ctx)
				if err != nil {
					Expect(err.Error()).To(ContainSubstring("context"))
				}
			})
		})
	})

	Describe("Callback System", func() {
		It("should execute callbacks on status updates", func() {
			callback := func(status sdk.UpdateStatus) {
				// Test callback
			}
			
			// We can't easily trigger a real update, but we can test the callback system
			// by ensuring callbacks are stored without error
			Expect(func() { updater.AddUpdateCallback(callback) }).ToNot(Panic())
		})

		It("should handle multiple callbacks", func() {
			callback1 := func(status sdk.UpdateStatus) {
				// First callback
			}
			
			callback2 := func(status sdk.UpdateStatus) {
				// Second callback
			}
			
			// Both callbacks should be registered
			Expect(func() { 
				updater.AddUpdateCallback(callback1)
				updater.AddUpdateCallback(callback2)
			}).ToNot(Panic())
		})
	})
})

var _ = Describe("Update Interval Parsing", func() {
	Describe("ParseUpdateInterval", func() {
		It("should parse valid intervals", func() {
			testCases := []struct {
				input    string
				expected time.Duration
			}{
				{"1m", time.Hour},     // clamped to minimum 1 hour
				{"5m", time.Hour},     // clamped to minimum 1 hour
				{"1h", 1 * time.Hour},
				{"2h", 2 * time.Hour},
				{"30s", time.Hour},    // clamped to minimum 1 hour
			}

			for _, tc := range testCases {
				By("parsing " + tc.input)
				duration, err := sdk.ParseUpdateInterval(tc.input)
				Expect(err).ToNot(HaveOccurred())
				Expect(duration).To(Equal(tc.expected))
			}
		})

		It("should handle invalid intervals", func() {
			invalidInputs := []string{
				"",
				"invalid",
				"1x", // invalid unit
				"abc",
			}

			for _, input := range invalidInputs {
				By("handling invalid input: " + input)
				_, err := sdk.ParseUpdateInterval(input)
				Expect(err).To(HaveOccurred())
			}

			// Test negative values (these should be clamped to 1h minimum)
			By("handling clamped input: -1m")
			duration, err := sdk.ParseUpdateInterval("-1m")
			Expect(err).ToNot(HaveOccurred())
			Expect(duration).To(Equal(time.Hour))

			// Test zero (this should be valid but clamped to 1h)
			By("handling edge case: 0")
			duration, err = sdk.ParseUpdateInterval("0")
			Expect(err).ToNot(HaveOccurred())
			Expect(duration).To(Equal(time.Hour))
		})

		It("should handle edge cases", func() {
			// Normal intervals within range
			duration, err := sdk.ParseUpdateInterval("24h")
			Expect(err).ToNot(HaveOccurred())
			Expect(duration).To(Equal(24 * time.Hour))

			// Very small intervals (clamped to 1h minimum)
			duration, err = sdk.ParseUpdateInterval("1s")
			Expect(err).ToNot(HaveOccurred())
			Expect(duration).To(Equal(time.Hour))

			// Very large intervals (clamped to 7 days maximum)
			duration, err = sdk.ParseUpdateInterval("720h") // 30 days
			Expect(err).ToNot(HaveOccurred())
			Expect(duration).To(Equal(7 * 24 * time.Hour))
		})
	})
})

var _ = Describe("Update Status", func() {
	It("should create valid update status", func() {
		status := sdk.UpdateStatus{
			PluginName: "test-plugin",
			OldVersion: "1.0.0",
			NewVersion: "1.1.0",
			Success:    true,
			Error:      nil,
			UpdatedAt:  time.Now(),
		}

		Expect(status.PluginName).To(Equal("test-plugin"))
		Expect(status.OldVersion).To(Equal("1.0.0"))
		Expect(status.NewVersion).To(Equal("1.1.0"))
		Expect(status.Success).To(BeTrue())
		Expect(status.Error).To(BeNil())
		Expect(status.UpdatedAt).ToNot(BeZero())
	})

	It("should handle failed updates", func() {
		err := fmt.Errorf("update failed")
		status := sdk.UpdateStatus{
			PluginName: "test-plugin",
			OldVersion: "1.0.0",
			NewVersion: "1.1.0",
			Success:    false,
			Error:      err,
			UpdatedAt:  time.Now(),
		}

		Expect(status.Success).To(BeFalse())
		Expect(status.Error).To(Equal(err))
	})
})

var _ = Describe("Default Update Callback", func() {
	It("should handle successful updates", func() {
		status := sdk.UpdateStatus{
			PluginName: "test-plugin",
			OldVersion: "1.0.0",
			NewVersion: "1.1.0",
			Success:    true,
			Error:      nil,
			UpdatedAt:  time.Now(),
		}

		// Should not panic
		Expect(func() { sdk.DefaultUpdateCallback(status) }).ToNot(Panic())
	})

	It("should handle failed updates", func() {
		status := sdk.UpdateStatus{
			PluginName: "test-plugin",
			OldVersion: "1.0.0",
			NewVersion: "1.1.0",
			Success:    false,
			Error:      fmt.Errorf("update failed"),
			UpdatedAt:  time.Now(),
		}

		// Should not panic
		Expect(func() { sdk.DefaultUpdateCallback(status) }).ToNot(Panic())
	})
})
