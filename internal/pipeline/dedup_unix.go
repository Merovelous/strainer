//go:build linux || darwin

package pipeline

import (
	"bufio"
	"bytes"
	"fmt"
	"math/bits"
	"os"
	"sync/atomic"
	"syscall"
)

const (
	mmapMaxSlots    = 256 << 20        // 256M slots × 8 bytes = 2 GB ceiling
	mmapMinSlots    = 1 << 20          // 1M slots × 8 bytes = 8 MB floor
	mmapMaxFileSize = int64(1<<32 - 1) // 4 GB − 1: uint32 offset limit
)

// mmapSlot references a line inside the mmap'd file.
// storedLength == 0 means the slot is empty.
// storedLength == N means the line has length N−1 (allows zero-length lines).
type mmapSlot struct {
	offset       uint32
	storedLength uint32
}

func canMmapDedup(fileSize int64) bool {
	return fileSize > 0 && fileSize <= mmapMaxFileSize
}

// mmapDedup deduplicates a plain file using mmap + open-addressing hash table.
// Every unique line is referenced by its offset in the mmap'd region — no
// string copies, no GC pressure. Accuracy is 100%: collisions are resolved
// by byte comparison against the mmap'd data.
func mmapDedup(p *Model, f *os.File, fileSize int64, writer *bufio.Writer) error {
	data, err := syscall.Mmap(int(f.Fd()), 0, int(fileSize), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return fmt.Errorf("mmap: %w", err)
	}
	defer syscall.Munmap(data)

	// Size table to ~50% load factor assuming ~10 bytes/line average.
	estLines := uint64(fileSize) / 10
	tableSize := mmapNextPow2(estLines * 2)
	if tableSize < mmapMinSlots {
		tableSize = mmapMinSlots
	}
	if tableSize > mmapMaxSlots {
		tableSize = mmapMaxSlots
	}
	table := make([]mmapSlot, tableSize)
	mask := tableSize - 1
	var usedSlots uint64

	pos := 0
	var tick int
	for pos < len(data) {
		tick++
		if tick&0xFFFF == 0 {
			select {
			case <-p.Ctx.Done():
				return nil
			default:
			}
		}

		rel := bytes.IndexByte(data[pos:], '\n')
		var line []byte
		var advance int
		if rel < 0 {
			line = data[pos:]
			advance = len(data) - pos
		} else {
			line = data[pos : pos+rel]
			advance = rel + 1
		}

		lineStart := uint32(pos)
		pos += advance

		atomic.AddInt64(&p.BytesRead, int64(advance))
		atomic.AddInt64(&p.LinesRead, 1)

		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}

		if !p.filterLine(line) {
			atomic.AddInt64(&p.LinesDropped, 1)
			continue
		}

		// Open-addressing lookup with linear probing.
		h := mmapFNV1a(line)
		idx := h & mask
		isDup := false
		stored := false
		for i := uint64(0); i < tableSize; i++ {
			s := &table[idx]
			if s.storedLength == 0 {
				// Empty slot — new unique line.
				usedSlots++
				if usedSlots > tableSize*8/10 {
					return fmt.Errorf("dedup table full (%d unique lines) — input has too many unique lines for available memory", usedSlots)
				}
				s.offset = lineStart
				s.storedLength = uint32(len(line)) + 1
				stored = true
				break
			}
			existing := data[s.offset : s.offset+s.storedLength-1]
			if bytes.Equal(existing, line) {
				isDup = true
				break
			}
			idx = (idx + 1) & mask
		}

		if !stored && !isDup {
			// Should not happen unless table is truly exhausted.
			return fmt.Errorf("dedup table exhausted")
		}

		if isDup {
			atomic.AddInt64(&p.LinesDropped, 1)
			continue
		}

		atomic.AddInt64(&p.LinesKept, 1)
		writer.Write(line)
		writer.WriteByte('\n')
		atomic.AddInt64(&p.BytesWritten, int64(len(line))+1)
	}

	return nil
}

func mmapFNV1a(b []byte) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for _, c := range b {
		h ^= uint64(c)
		h *= prime64
	}
	return h
}

func mmapNextPow2(n uint64) uint64 {
	if n <= 1 {
		return 1
	}
	return 1 << bits.Len64(n-1)
}
