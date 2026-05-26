//go:build windows

package tui

import "runtime"

func getCPURawTicks() float64 { return 0 } // WinAPI required; not implemented

func getRSSBytes() int64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return int64(ms.Sys)
}

func cpuTicksPerSec() float64 { return 100.0 }
