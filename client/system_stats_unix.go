//go:build !windows
// +build !windows

package client

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
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

// getOSProcessListImpl returns list of processes on Unix/Linux/Mac
func getOSProcessListImpl() []OSProcess {
	var processes []OSProcess

	procs, err := process.Processes()
	if err != nil {
		return processes
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}

		cpuPercent, err := p.CPUPercent()
		if err != nil {
			cpuPercent = 0
		}

		memInfo, err := p.MemoryInfo()
		if err != nil || memInfo == nil {
			continue
		}

		// Convert bytes to MB
		memMB := float64(memInfo.RSS) / (1024 * 1024)

		processes = append(processes, OSProcess{
			Name:   name,
			PID:    int(p.Pid),
			CPU:    cpuPercent,
			Memory: memMB,
		})
	}

	return processes
}
