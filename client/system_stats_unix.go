//go:build !windows
// +build !windows

package client

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// getSafeSystemStats returns system stats safely (Unix/Linux/Mac)
func getSafeSystemStats() map[string]float64 {
	stats := make(map[string]float64)

	if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
		stats["cpu"] = cpuPercent[0]
	}

	if memStats, err := mem.VirtualMemory(); err == nil && memStats != nil {
		stats["mem"] = memStats.UsedPercent
	}

	if diskStats, err := disk.Usage("/"); err == nil && diskStats != nil {
		stats["disk"] = diskStats.UsedPercent
	}

	return stats
}
