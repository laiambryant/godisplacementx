# SIMD compositing kernels (`simd/archsimd`)

`godisplacementx` has an **experimental** SIMD variant of the `internal/gen`
pixel kernels, built on Go 1.26's `simd/archsimd` package. It is opt-in at
compile time; the default build is unchanged (pure Go, no `GOEXPERIMENT`, cross-
compiles to every GOARCH).

## Building & running

```sh
make cli-simd        # build the SIMD CLI  -> build/bin/godisplacementx-cli-simd
make test-simd       # go test ./... with GOEXPERIMENT=simd -tags simd
make bench           # benchmarks, scalar build
make bench-simd      # benchmarks, simd build
make bench-compare   # benchstat scalar vs simd (needs benchstat)
```

`godisplacementx-cli-simd version` reports the active variant:

```
godisplacementx <ver> (simd)     # vs "(scalar)" for the default build
```

## Constraints

- **`GOEXPERIMENT=simd` is required at build time.** `simd/archsimd` only exists
  under that experiment; the Makefile threads it into the `*-simd` targets.
- **amd64 / AVX2 only.** `archsimd` is amd64-specific. The kernels target the
  broad **AVX2** baseline and deliberately avoid AVX-512-only operations
  (mask registers, `VPMOV*` narrowing, unsigned int→float conversion). A runtime
  `archsimd.X86.AVX2()` check falls back to scalar on older CPUs.
- **arm64 (and any non-amd64) `-tags simd` build** compiles too: the SIMD
  symbols alias the scalar kernels (`kernels_dispatch_simd_fallback.go`), so the
  arm64 release targets keep working with a correct scalar fallback.
- **Tolerance ±1.** The SIMD path uses `float32` lanes vs the scalar reference's
  `float64`, so results may differ by at most 1 per channel. The default golden
  tests are untouched; `kernels_simd_test.go` asserts the ±1 bound.

## What is vectorized (and what isn't)

Vectorization was **benchmark-driven**. Results on the dev machine (Ryzen-class,
AVX2, no AVX-512), ~1024²:

| Kernel                    | Scalar    | SIMD      | Result             | Production dispatch |
|---------------------------|-----------|-----------|--------------------|---------------------|
| `Fill`                    | ~620 µs   | ~98 µs    | **~6× faster**     | **SIMD**            |
| `Invert`                  | ~870 µs   | ~130 µs   | **~6.6× faster**   | **SIMD**            |
| `Blend` — float, any mode | ~5–15 ms  | ~150 ms   | ~25–40× **slower** | **scalar**          |
| `Blend` — integer, source-over (one wide run) | ~340 µs / 256² | ~100–180 µs | **~2–3.5× faster** | **scalar** (see below) |

`Fill` and `Invert` are wide (32-byte) byte-lane passes with no numeric
conversion — a natural SIMD win. The per-pixel compositor `blendInto` is the
opposite: it needs `uint8↔float32` conversions and, on AVX2 without wide byte
narrowing, can only process 2 pixels per step. The long `archsimd` method chain
plus the conversions make it far slower than the tight scalar `float64` loop, and
end-to-end `Generate` regresses ~24×.

**Integer source-over (`blendSourceOverSIMD`).** Source-over onto an opaque
backdrop reduces, per byte, to an affine map `out = round(A + B·pix)` with
`A = g·255·sa` and `B = 1−sa` constant for the run, so it can be done in
fixed-point `uint16` lanes with **no float conversion** (Q8: `out = (Bf·pix + Af)
>> 8`). AVX2 has no `uint16→uint8` pack in this `archsimd` (the `VPMOV*` narrowing
is AVX-512-only; `VPACKUSWB` isn't exposed), so the low byte of each lane is
gathered with a `VPSHUFB` shuffle and written with a single 8-byte store — no
scalar per-byte write-back (that write-back is what stalls the float
`blendRunSIMD`). On a **single wide run** this is ~2–3.5× faster than scalar
(compute-bound, cache-resident: `BenchmarkSourceOverL2AB`).

Yet it still stays **off the production path**, for a different reason than the
float kernel: `FillRect` calls `blendRun` once per **scanline** of every
primitive, and the real draw loop is dominated by *many short runs* (grid / line /
column cells a few dozen pixels wide). There the per-call vector setup (building
the coefficient and shuffle-index vectors) is not amortised, and wiring it into
`blendRun` regresses end-to-end `Generate` ~15×. A production win would require
hoisting that setup up to `FillRect` (computed once per rect, not per row), or
otherwise batching the draw loop into wide runs — a larger refactor. Note also
that on a thermally-limited laptop, sustained heavy-AVX2 blend triggers power
throttling that a memory-bound wide run cannot escape; the ~2–3.5× figure is the
un-throttled, compute-bound speed.

So production keeps the **scalar** compositor and uses SIMD only for the
`Fill`/`Invert` passes. Both `blendRunSIMD` (float, all 16 modes; separable modes
via lane math + masks, the non-separable / special modes — source-atop, xor,
lighter, luminosity — fall back to scalar) and `blendSourceOverSIMD` (integer,
source-over) are exercised by the A/B benchmarks and tolerance tests and document
*why* the scalar path wins. `ApplyColor` (256-entry LUT gather) and `ApplyNormal`
(strided neighbour access) have no clean AVX2 lowering and stay scalar.

## Code layout

Batch kernels operate on a run of pixels and are selected by build tag:

| File | Build tag | Role |
|------|-----------|------|
| `kernels_scalar.go` | *(none)* | scalar reference; always compiled |
| `kernels_dispatch_nosimd.go` | `!simd` | default dispatch → scalar |
| `kernels_dispatch_simd.go` | `simd && amd64` | AVX2 kernels + dispatch |
| `kernels_dispatch_simd_fallback.go` | `simd && !amd64` | SIMD symbols alias scalar |
| `variant_{no,}simd.go` | `!simd` / `simd` | `BuildVariant` string |

Callers (`canvas.go` `Fill`/`FillRect`, `post.go` `ApplyInvert`/`ApplyColor`/
`ApplyNormal`) invoke the dispatch wrappers (`fillRun`, `blendRun`, …). Keeping
the scalar kernels always compiled lets the single-binary A/B benchmarks
(`kernels_simd_bench_test.go`) call scalar and SIMD side by side.

## Benchmarks

- `kernels_bench_test.go` (untagged) benchmarks the dispatch wrappers and the
  end-to-end `Generate`, so the same benchmark measures scalar in the default
  build and SIMD under `-tags simd`. `make bench-compare` diffs the two with
  benchstat (the "two-build" comparison).
- `kernels_simd_bench_test.go` (`//go:build simd`) has the single-binary A/B
  benchmarks that call `*Scalar` and `*SIMD` directly (`-bench AB`).
  `BenchmarkSourceOverAB` (main-memory-sized) and `BenchmarkSourceOverL2AB`
  (cache-resident, compute-bound) compare the integer source-over kernel against
  scalar; run the L2 one to see the un-throttled per-run speed.

## Possible future work

- The integer fixed-point source-over kernel (`blendSourceOverSIMD`) already makes
  the *per-run* compositor ~2–3.5× faster than scalar (no float conversion). What
  blocks it from production is the **per-call setup on short runs**, not the
  per-pixel math: `blendRun` is invoked once per scanline. Hoisting the coefficient
  and shuffle-index vector setup up to `FillRect` (compute once per rect, reuse
  across its rows), or restructuring the draw loop to hand the kernel wide runs,
  would let the win reach `Generate`. Until then the kernel documents the approach.
- Re-evaluate on AVX-512 hardware, where the `uint16→uint8` narrowing (here a
  `VPSHUFB` gather + partial store) becomes a single `VPMOVUSWB`, and the wider
  conversion/narrowing ops remove the main AVX2 bottleneck.
