//go:build !windows
// +build !windows

package client

import (
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"mww2.com/server_manager/common"
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

// getSystemInfoImpl returns system information on Unix/Linux/Mac
func getSystemInfoImpl() *common.SystemInfoPayload {
	info := &common.SystemInfoPayload{}

	// Get hostname
	hostname, _ := os.Hostname()
	info.Hostname = hostname

	// Get OS and architecture
	info.OS = runtime.GOOS
	info.Arch = runtime.GOARCH

	// Get CPU count
	cpuCount, _ := cpu.Counts(false)
	info.CPUCount = cpuCount

	// Get memory info
	if memStats, err := mem.VirtualMemory(); err == nil && memStats != nil {
		info.TotalMemory = memStats.Total
		info.AvailMemory = memStats.Available
		info.UsedMemory = memStats.Used
		info.MemoryPercent = memStats.UsedPercent
	}

	// Get uptime
	if uptime, err := host.Uptime(); err == nil {
		info.Uptime = uptime
	}

	// Get disk info
	if diskStats, err := disk.Usage("/"); err == nil && diskStats != nil {
		info.DiskTotal = diskStats.Total
		info.DiskUsed = diskStats.Used
		info.DiskFree = diskStats.Free
		info.DiskPercent = diskStats.UsedPercent
	}

	return info
}
