//go:build darwin

package pipeline

import (
	"os/exec"
	"strconv"
	"strings"
)

// AvailableRAM returns available RAM in bytes and whether the query succeeded.
func AvailableRAM() (int64, bool) {
	cmd := exec.Command("sysctl", "-n", "hw.memsize")
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	total, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, false
	}
	// hw.memsize is total physical RAM; use 60% as a conservative available estimate.
	return int64(float64(total) * 0.6), true
}
