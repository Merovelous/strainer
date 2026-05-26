//go:build ignore

// gen_bench generates a reproducible synthetic wordlist for benchmarking.
// Usage:
//
//	go run gen_bench.go [-size 10000] [-out bench.txt]
//
// -size: approximate output size in MB (default 500)
// -out:  output file path (default bench.txt)
//
// The generator uses a fixed seed (42) so the same -size always produces
// the same byte sequence. Line composition (by count):
//
//	40% valid:     printable ASCII, length 8–12   ← kept by the benchmark filter
//	20% too-short: printable ASCII, length 1–7    ← dropped by min-length
//	20% too-long:  printable ASCII, length 13–30  ← dropped by max-length
//	20% non-ASCII: contains bytes outside 0x20–0x7E ← dropped by ascii-only
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()-_=+[]{}|;:,.<>?"

func main() {
	sizeMB := flag.Int("size", 500, "approximate output size in MB")
	out := flag.String("out", "bench.txt", "output file path")
	flag.Parse()

	rng := rand.New(rand.NewSource(42))

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 4*1024*1024)
	target := int64(*sizeMB) * 1024 * 1024
	var written int64

	lineKind := 0
	for written < target {
		var length int
		var nonASCII bool

		switch lineKind % 5 {
		case 0, 1: // 40% valid (8–12, ASCII)
			length = 8 + rng.Intn(5)
		case 2: // 20% too short (1–7, ASCII)
			length = 1 + rng.Intn(7)
		case 3: // 20% too long (13–30, ASCII)
			length = 13 + rng.Intn(18)
		case 4: // 20% non-ASCII
			length = 6 + rng.Intn(10)
			nonASCII = true
		}
		lineKind++

		buf := make([]byte, length)
		for i := range buf {
			if nonASCII && i == length/2 {
				// Inject a non-printable byte in the middle
				buf[i] = byte(0x80 + rng.Intn(0x7F))
			} else {
				buf[i] = charset[rng.Intn(len(charset))]
			}
		}
		w.Write(buf)
		w.WriteByte('\n')
		written += int64(length) + 1
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintln(os.Stderr, "flush error:", err)
		os.Exit(1)
	}

	fmt.Printf("wrote %d MB to %s\n", written/1024/1024, *out)
}
