//go:build windows
// +build windows

package client

import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
	"gorat/common"
)

var (
	gopsutilInitialized bool
	gopsutilMutex       sync.Mutex
)

// getSafeSystemStats returns system stats safely on Windows
// Uses panic recovery to handle gopsutil failures
func getSafeSystemStats() map[string]float64 {
	stats := make(map[string]float64)

	gopsutilMutex.Lock()
	defer gopsutilMutex.Unlock()

	if !gopsutilInitialized {
		log.Printf("[DEBUG] getSafeSystemStats: First call, checking gopsutil")
		gopsutilInitialized = true
	}

	// Wrap each gopsutil call in its own recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[WARN] getSafeSystemStats: Panic during stats collection: %v", r)
		}
	}()

	// CPU stats
	func() {
		defer func() { recover() }()
		if cpuPercent, err := cpu.Percent(time.Second, false); err == nil && len(cpuPercent) > 0 {
			stats["cpu"] = cpuPercent[0]
		}
	}()

	// Memory stats
	func() {
		defer func() { recover() }()
		if memStats, err := mem.VirtualMemory(); err == nil && memStats != nil {
			stats["mem"] = memStats.UsedPercent
		}
	}()

	// Disk stats
	func() {
		defer func() { recover() }()
		if diskStats, err := disk.Usage("C:\\"); err == nil && diskStats != nil {
			stats["disk"] = diskStats.UsedPercent
		}
	}()

	return stats
}

// getOSProcessListImpl returns list of processes on Windows
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

// getSystemInfoImpl returns system information on Windows
func getSystemInfoImpl() *common.SystemInfoPayload {
	info := &common.SystemInfoPayload{}

	// Get hostname
	if hostname, err := os.Hostname(); err == nil {
		info.Hostname = hostname
	}

	// Get OS and architecture
	info.OS = runtime.GOOS
	info.Arch = runtime.GOARCH

	// Get CPU count
	if count, err := cpu.Counts(false); err == nil {
		info.CPUCount = count
	}

	// Get memory stats
	if mem, err := mem.VirtualMemory(); err == nil {
		info.TotalMemory = mem.Total
		info.AvailMemory = mem.Available
		info.UsedMemory = mem.Used
		info.MemoryPercent = mem.UsedPercent
	}

	// Get uptime
	if uptime, err := host.Uptime(); err == nil {
		info.Uptime = uptime
	}

	// Get disk stats for C: drive
	if disk, err := disk.Usage("C:\\"); err == nil {
		info.DiskTotal = disk.Total
		info.DiskUsed = disk.Used
		info.DiskFree = disk.Free
		info.DiskPercent = disk.UsedPercent
	}

	return info
}
