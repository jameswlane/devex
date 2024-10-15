package sysmetrics

import (
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"time"
)

func GetCPUUsage() (float64, error) {
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}
	return cpuPercent[0], nil
}

func GetRAMUsage() (float64, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return vmStat.UsedPercent, nil
}

func GetDiskUsage() (float64, error) {
	diskStat, err := disk.Usage("/")
	if err != nil {
		return 0, err
	}
	return diskStat.UsedPercent, nil
}
