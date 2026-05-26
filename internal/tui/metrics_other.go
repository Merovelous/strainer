//go:build !linux && !darwin && !windows

package tui

func getCPURawTicks() float64  { return 0 }
func getRSSBytes() int64       { return 0 }
func cpuTicksPerSec() float64  { return 100.0 }
