package gen

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math"
	"math/rand/v2"
)

// RNG mirrors the semantics of the original src/utils/random.ts so the Go port
// reproduces the same drawing logic. It is seedable for reproducible output.
type RNG struct {
	r *rand.Rand
}

// NewRNG returns a deterministic RNG for the given seed.
func NewRNG(seed uint64) *RNG {
	// Two distinct streams for PCG; derive the second from the first.
	return &RNG{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// NewRandomRNG returns an RNG seeded from a cryptographically random source,
// matching the original's unseeded Math.random() behaviour (each run differs).
// crypto/rand.Read never fails (guaranteed since Go 1.24).
func NewRandomRNG() *RNG {
	var b [16]byte
	_, _ = cryptorand.Read(b[:])
	s1 := binary.LittleEndian.Uint64(b[0:8])
	s2 := binary.LittleEndian.Uint64(b[8:16])
	return &RNG{r: rand.New(rand.NewPCG(s1, s2))}
}

// Float returns a float in [0, 1), like JS Math.random().
func (g *RNG) Float() float64 { return g.r.Float64() }

// Boolean mirrors randomBoolean: Math.random() >= 0.5.
func (g *RNG) Boolean() bool { return g.Float() >= 0.5 }

// Integer mirrors randomInteger(min, max): inclusive on both ends.
// floor(random() * (floor(max) - ceil(min) + 1) + ceil(min)).
func (g *RNG) Integer(min, max int) int {
	return integerFrom(g.Float(), min, max)
}

// integerFrom is the pure randomInteger formula, factored out for testing.
func integerFrom(f float64, min, max int) int {
	mn := math.Ceil(float64(min))
	mx := math.Floor(float64(max))
	return int(math.Floor(f*(mx-mn+1) + mn))
}
