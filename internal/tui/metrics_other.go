//go:build !linux

package tui

func getCPURawTicks() float64 { return 0 }
func getRSSBytes() int64      { return 0 }
