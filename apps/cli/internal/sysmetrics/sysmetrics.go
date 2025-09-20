package sysmetrics

import (
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var (
	GetCPULoad     = cpu.Percent
	GetMemoryStats = mem.VirtualMemory
	GetDiskUsage   = disk.Usage
)

func GetCPULoadPercent(interval time.Duration) ([]float64, error) {
	return GetCPULoad(interval, false)
}

func GetMemoryUsagePercent() (*mem.VirtualMemoryStat, error) {
	return GetMemoryStats()
}

func GetDiskUsagePercent(path string) (*disk.UsageStat, error) {
	return GetDiskUsage(path)
}
