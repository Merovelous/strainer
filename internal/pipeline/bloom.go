package pipeline

import "math"

const bloomK = 7 // hash functions — optimal for ~1% load factor

type bloomFilter struct {
	bits []uint64
	m    uint64 // total bit count (always a multiple of 64)
}

func newBloomFilter(sizeBytes int64) *bloomFilter {
	words := uint64(sizeBytes) / 8
	if words == 0 {
		words = 1
	}
	return &bloomFilter{
		bits: make([]uint64, words),
		m:    words * 64,
	}
}

// seenAndAdd returns true if line was previously seen, and always records it.
func (bf *bloomFilter) seenAndAdd(line []byte) bool {
	h1 := bloomFNV1a(line)
	h2 := bloomFNV1(line)
	if h2 == 0 {
		h2 = 1
	}
	seen := true
	for i := uint64(0); i < bloomK; i++ {
		pos := (h1 + i*h2) % bf.m
		mask := uint64(1) << (pos & 63)
		if bf.bits[pos>>6]&mask == 0 {
			seen = false
		}
		bf.bits[pos>>6] |= mask
	}
	return seen
}

func bloomFNV1a(b []byte) uint64 {
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

// bloomFNV1 is the multiplicative-before-xor variant, giving an independent hash.
func bloomFNV1(b []byte) uint64 {
	const (
		offset64 uint64 = 14695981039346656037
		prime64  uint64 = 1099511628211
	)
	h := offset64
	for _, c := range b {
		h *= prime64
		h ^= uint64(c)
	}
	return h
}

// BloomFPR estimates the false-positive rate.
// sizeBytes: filter size; estLines: estimated unique element count.
func BloomFPR(sizeBytes, estLines int64) float64 {
	if sizeBytes <= 0 || estLines <= 0 {
		return 0
	}
	m := float64(sizeBytes) * 8
	n := float64(estLines)
	k := float64(bloomK)
	return math.Pow(1-math.Exp(-k*n/m), k)
}
