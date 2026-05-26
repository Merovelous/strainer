//go:build !linux && !darwin

package main

import (
	"bufio"
	"os"
)

func canMmapDedup(_ int64) bool { return false }

func mmapDedup(_ *pipelineModel, _ *os.File, _ int64, _ *bufio.Writer) error {
	panic("mmapDedup: unreachable on this platform")
}
