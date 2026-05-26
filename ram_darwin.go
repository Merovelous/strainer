//go:build darwin

package main

import (
	"strconv"
	"strings"
)

func availableRAM() (int64, bool) {
	out, err := runCommand("sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0, false
	}
	total, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return 0, false
	}
	// hw.memsize is total physical RAM; use 60% as a conservative available estimate.
	return int64(float64(total) * 0.6), true
}
