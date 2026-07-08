//go:build !simd

package gen

// BuildVariant names the compositing kernel variant compiled into the binary.
// Reported by `godisplacementx version`.
const BuildVariant = "scalar"
