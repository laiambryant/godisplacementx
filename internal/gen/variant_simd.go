//go:build simd

package gen

// BuildVariant names the compositing kernel variant compiled into the binary.
// Reported by `godisplacementx version`. The "simd" build uses simd/archsimd
// (AVX2) kernels on amd64 and falls back to scalar on other architectures.
const BuildVariant = "simd"
