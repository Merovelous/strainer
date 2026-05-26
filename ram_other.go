//go:build !linux && !darwin

package main

func availableRAM() (int64, bool) {
	return 0, false
}
