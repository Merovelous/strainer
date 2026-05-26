//go:build !linux && !darwin

package pipeline

import (
	"bufio"
	"os"
)

const mmapMaxFileSize = int64(1<<32 - 1) // mirrors dedup_unix.go; mmap never used on this platform

func canMmapDedup(_ int64) bool { return false }

func mmapDedup(_ *Model, _ *os.File, _ int64, _ *bufio.Writer) error {
	panic("mmapDedup: unreachable on this platform")
}
