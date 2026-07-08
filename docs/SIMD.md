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

| Kernel        | Scalar    | SIMD      | Result             | Production dispatch |
|---------------|-----------|-----------|--------------------|---------------------|
| `Fill`        | ~620 µs   | ~98 µs    | **~6× faster**     | **SIMD**            |
| `Invert`      | ~870 µs   | ~130 µs   | **~6.6× faster**   | **SIMD**            |
| `Blend` (any) | ~5–15 ms  | ~150 ms   | ~25–40× **slower** | **scalar**          |

`Fill` and `Invert` are wide (32-byte) byte-lane passes with no numeric
conversion — a natural SIMD win. The per-pixel compositor `blendInto` is the
opposite: it needs `uint8↔float32` conversions and, on AVX2 without wide byte
narrowing, can only process 2 pixels per step. The long `archsimd` method chain
plus the conversions make it far slower than the tight scalar `float64` loop, and
end-to-end `Generate` regresses ~24×.

So production keeps the **scalar** compositor and uses SIMD only for the
`Fill`/`Invert` passes. `blendRunSIMD` still exists and **vectorizes all 16 blend
modes correctly** (separable modes via lane math + masks; the non-separable /
special modes — source-atop, xor, lighter, luminosity — fall back to scalar). It
is exercised by the A/B benchmarks and the tolerance tests, and documents *why*
the scalar path wins. `ApplyColor` (256-entry LUT gather) and `ApplyNormal`
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

## Possible future work

- A dedicated integer fixed-point AVX2 path for the dominant source-over
  fast-path (wide `uint16` lanes, no float conversion) could make the compositor
  competitive; the current float path is kept for full 16-mode coverage.
- Re-evaluate on AVX-512 hardware, where wide narrowing/conversion ops remove the
  main bottleneck.
