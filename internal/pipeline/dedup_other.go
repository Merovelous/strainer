//go:build !linux && !darwin

package pipeline

import (
	"bufio"
	"os"
)

func canMmapDedup(_ int64) bool { return false }

func mmapDedup(_ *Model, _ *os.File, _ int64, _ *bufio.Writer) error {
	panic("mmapDedup: unreachable on this platform")
}
