package lexicon

import (
	"crypto/sha256"
	"encoding/binary"
)

// SeededIndex deterministically maps (seed, slot) to an index in [0, modulus).
//
// Algorithm (must be byte-for-byte identical across Go/Python/JS for cross-
// language parity tests):
//
//  1. Compose: seed_utf8_bytes || big-endian 8-byte uint64 of slot
//  2. Hash:    SHA-256
//  3. Read:    first 8 bytes of the digest, big-endian, as uint64
//  4. Map:     value mod modulus
//
// The slot parameter lets a recipe with multiple roll positions derive
// independent indices from a single seed. If modulus is 0, returns 0.
func SeededIndex(seed string, slot uint64, modulus int) int {
	if modulus <= 0 {
		return 0
	}
	h := sha256.New()
	h.Write([]byte(seed))
	var slotBytes [8]byte
	binary.BigEndian.PutUint64(slotBytes[:], slot)
	h.Write(slotBytes[:])
	digest := h.Sum(nil)
	v := binary.BigEndian.Uint64(digest[:8])
	return int(v % uint64(modulus))
}
