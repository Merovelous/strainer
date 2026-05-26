package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SevenZipBin is detected once at startup — "7zz" on macOS, "7z" on Linux.
var SevenZipBin = func() string {
	for _, name := range []string{"7z", "7zz", "7za"} {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return "7z" // fall through to a clear error message at runtime
}()

var archiveExts = map[string]bool{
	".7z": true, ".zip": true, ".tar": true, ".gz": true,
	".bz2": true, ".xz": true, ".tar.gz": true, ".tar.bz2": true,
	".tar.xz": true, ".tgz": true, ".rar": true,
}

// IsArchiveFile returns true if name has a recognized archive extension.
func IsArchiveFile(name string) bool {
	lower := strings.ToLower(name)
	// Check compound extensions first
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return archiveExts[filepath.Ext(lower)]
}

// HumanSize formats a byte count as a human-readable string.
func HumanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// CommaFmt formats an integer with comma separators.
func CommaFmt(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// FormatDuration formats a duration as MM:SS or HH:MM:SS.
func FormatDuration(d time.Duration) string {
	s := int(d.Seconds())
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

// HumanSpeed formats a bytes-per-second rate as a human-readable string.
func HumanSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else if bytesPerSec < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB/s", bytesPerSec/(1024*1024*1024))
}

// IsLikelyBinary returns true if the file looks like binary data.
// It reads up to 8 KB and checks null-byte density: real binaries (ELF,
// AppImage, PE) have hundreds of nulls in their headers, while dirty
// wordlists that happen to contain a few \x00 bytes stay well below the
// threshold (0.3% ≈ 25 nulls in 8 KB).
func IsLikelyBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	if n == 0 {
		return false
	}
	var nulls int
	for _, b := range buf[:n] {
		if b == 0x00 {
			nulls++
		}
	}
	// ELF/AppImage/PE headers have 20+ null bytes in the first 64 bytes.
	// A dirty wordlist with a few \x00 entries will typically have 1–2.
	return nulls >= 8
}

// ParseBloomSize parses human size strings like "16g", "2048m" into bytes.
func ParseBloomSize(s string) int64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) < 2 {
		return 0
	}
	n, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	switch s[len(s)-1] {
	case 'g':
		return n << 30
	case 'm':
		return n << 20
	}
	return 0
}
