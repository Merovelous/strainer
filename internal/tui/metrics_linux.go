//go:build linux

// cpuTicksPerSec is the scale used to convert getCPURawTicks deltas to seconds.
// Linux /proc/self/stat reports jiffies; HZ=100 on virtually all kernels.

package tui

import (
	"os"
	"strconv"
	"strings"
)

func getCPURawTicks() float64 {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return 0
	}
	utime, _ := strconv.ParseFloat(fields[13], 64)
	stime, _ := strconv.ParseFloat(fields[14], 64)
	return utime + stime
}

func getRSSBytes() int64 {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseInt(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

func cpuTicksPerSec() float64 { return 100.0 }
