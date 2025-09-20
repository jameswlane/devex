package sysmetrics_test

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/sysmetrics"
)

var _ = Describe("GetCPULoadPercent", func() {
	It("retrieves CPU usage successfully", func() {
		sysmetrics.GetCPULoad = func(interval time.Duration, percpu bool) ([]float64, error) {
			return []float64{25.5}, nil
		}

		load, err := sysmetrics.GetCPULoadPercent(0)
		Expect(err).ToNot(HaveOccurred())
		Expect(load).To(Equal([]float64{25.5}))
	})

	It("returns an error if CPU usage retrieval fails", func() {
		sysmetrics.GetCPULoad = func(interval time.Duration, percpu bool) ([]float64, error) {
			return nil, fmt.Errorf("mock error")
		}

		_, err := sysmetrics.GetCPULoadPercent(0)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock error"))
	})
})

var _ = Describe("GetMemoryUsagePercent", func() {
	It("retrieves memory usage successfully", func() {
		sysmetrics.GetMemoryStats = func() (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{UsedPercent: 42.7}, nil
		}

		usage, err := sysmetrics.GetMemoryUsagePercent()
		Expect(err).ToNot(HaveOccurred())
		Expect(usage.UsedPercent).To(Equal(42.7))
	})

	It("returns an error if memory usage retrieval fails", func() {
		sysmetrics.GetMemoryStats = func() (*mem.VirtualMemoryStat, error) {
			return nil, fmt.Errorf("mock error")
		}

		_, err := sysmetrics.GetMemoryUsagePercent()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock error"))
	})
})

var _ = Describe("GetDiskUsagePercent", func() {
	It("retrieves disk usage successfully", func() {
		sysmetrics.GetDiskUsage = func(path string) (*disk.UsageStat, error) {
			return &disk.UsageStat{UsedPercent: 65.3}, nil
		}

		usage, err := sysmetrics.GetDiskUsagePercent("/")
		Expect(err).ToNot(HaveOccurred())
		Expect(usage.UsedPercent).To(Equal(65.3))
	})

	It("returns an error if disk usage retrieval fails", func() {
		sysmetrics.GetDiskUsage = func(path string) (*disk.UsageStat, error) {
			return nil, fmt.Errorf("mock error")
		}

		_, err := sysmetrics.GetDiskUsagePercent("/")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock error"))
	})
})
