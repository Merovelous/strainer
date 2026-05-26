//go:build !linux

package main

func getCPURawTicks() float64 { return 0 }
func getRSSBytes() int64      { return 0 }
