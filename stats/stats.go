package stats

import (
	"Regex/logger"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

// SystemStats holds the current system metrics.
type SystemStats struct {
	CPUUsage       float64 `json:"cpu_usage"`
	RAMUsageGB     float64 `json:"ram_usage_gb"`
	DiskReadMBps   float64 `json:"disk_read_mbps"`
	DiskWriteMBps  float64 `json:"disk_write_mbps"`
	QueueDepth     int     `json:"queue_depth"`
}

var (
	lastDiskRead  uint64
	lastDiskWrite uint64
	lastDiskTime  time.Time
	diskStatsLock sync.Mutex
)

// GetSystemStats returns the current system metrics.
func GetSystemStats(appLogger *logger.AppLogger) (*SystemStats, error) {
	// CPU Usage
	percentages, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}
	cpuUsage := 0.0
	if len(percentages) > 0 {
		cpuUsage = percentages[0]
	}

	// RAM Usage
	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	ramUsedGB := float64(vm.Used) / (1024 * 1024 * 1024)

	// Disk I/O
	diskStatsLock.Lock()
	defer diskStatsLock.Unlock()

	ioCounters, err := disk.IOCounters()
	if err != nil {
		return nil, err
	}

	var totalRead, totalWrite uint64
	for _, counter := range ioCounters {
		totalRead += counter.ReadBytes
		totalWrite += counter.WriteBytes
	}

	var readSpeed, writeSpeed float64
	now := time.Now()
	if !lastDiskTime.IsZero() {
		duration := now.Sub(lastDiskTime).Seconds()
		if duration > 0 {
			readSpeed = float64(totalRead-lastDiskRead) / (1024 * 1024) / duration
			writeSpeed = float64(totalWrite-lastDiskWrite) / (1024 * 1024) / duration
		}
	}

	lastDiskRead = totalRead
	lastDiskWrite = totalWrite
	lastDiskTime = now

	return &SystemStats{
		CPUUsage:      cpuUsage,
		RAMUsageGB:    ramUsedGB,
		DiskReadMBps:  readSpeed,
		DiskWriteMBps: writeSpeed,
	}, nil
}

// getDriveLetter returns the drive where the executable is running.
func getDriveLetter(appLogger *logger.AppLogger) string {
	exePath, err := os.Executable()
	if err != nil {
		appLogger.Warn("Could not get executable path for disk usage", "error", err)
		return "C:\\" // Fallback for Windows
	}
	drive := filepath.VolumeName(exePath)
	if drive == "" {
		return "/" // Fallback for Unix-like systems
	}
	drive = drive + "\\"
	return drive
}
