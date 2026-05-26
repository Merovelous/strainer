//go:build !linux && !darwin

package pipeline

// AvailableRAM returns available RAM in bytes and whether the query succeeded.
func AvailableRAM() (int64, bool) {
	return 0, false
}
