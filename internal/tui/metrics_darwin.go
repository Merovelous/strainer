//go:build darwin

package tui

import "syscall"

// getCPURawTicks returns cumulative CPU time in microseconds (user + sys).
func getCPURawTicks() float64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	u := float64(ru.Utime.Sec)*1e6 + float64(ru.Utime.Usec)
	s := float64(ru.Stime.Sec)*1e6 + float64(ru.Stime.Usec)
	return u + s
}

// getRSSBytes returns current resident set size in bytes.
// ru_maxrss is bytes on macOS (unlike Linux where it is KB).
func getRSSBytes() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return int64(ru.Maxrss)
}

func cpuTicksPerSec() float64 { return 1e6 } // microseconds per second
