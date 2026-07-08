# Build system for godisplacementx (replaces build.ps1).
#
# Recipes are POSIX sh. On Windows run them from Git Bash (or WSL) with `make`
# installed (e.g. `choco install make` / `scoop install make`); on Linux/macOS
# and CI they run as-is.
#
# The default build is pure Go (identical to before) and cross-compiles to every
# GOARCH. The SIMD variant (`*-simd` targets) adds GOEXPERIMENT=simd + `-tags
# simd`: on amd64 it uses simd/archsimd (AVX2) for the fill/invert passes; on
# other arches it falls back to scalar. See docs/SIMD.md.

GO ?= go
BIN_DIR := build/bin
DIST_DIR := dist
BENCH_PKG := ./internal/gen

GOOS := $(shell $(GO) env GOOS)
EXE :=
ifeq ($(GOOS),windows)
EXE := .exe
endif

VERSION := $(patsubst v%,%,$(shell git describe --tags --always 2>/dev/null || echo 0.0.0-dev))
LDFLAGS := -s -w -X godisplacementx/internal/cli.Version=$(VERSION)
GOPATH_BIN := $(shell $(GO) env GOPATH)/bin

# Benchmark knobs (override on the command line, e.g. `make bench BENCH=Blend`).
BENCH ?= .
BENCHTIME ?= 1s
COUNT ?= 6

.PHONY: all help cli cli-simd gui test test-simd bench bench-simd bench-compare lint sprites clean release

all: cli gui

help:
	@echo "targets:"
	@echo "  cli           build the pure-Go CLI            -> $(BIN_DIR)/godisplacementx-cli$(EXE)"
	@echo "  cli-simd      build the SIMD CLI (GOEXPERIMENT) -> $(BIN_DIR)/godisplacementx-cli-simd$(EXE)"
	@echo "  gui           build the Wails desktop app (host-only)"
	@echo "  test          go test ./..."
	@echo "  test-simd     go test ./... with -tags simd"
	@echo "  bench         run benchmarks (scalar build)"
	@echo "  bench-simd    run benchmarks (simd build)"
	@echo "  bench-compare benchstat scalar vs simd (needs benchstat)"
	@echo "  lint          golangci-lint run ./..."
	@echo "  sprites       regenerate sprite PNGs (needs Node)"
	@echo "  release       cross-compile CLI archives into $(DIST_DIR)/"
	@echo "  clean         remove build/dist/bench artifacts"

cli:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/godisplacementx-cli$(EXE) .
	@echo "built $(BIN_DIR)/godisplacementx-cli$(EXE) ($(VERSION), scalar)"

cli-simd:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOEXPERIMENT=simd $(GO) build -tags simd -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/godisplacementx-cli-simd$(EXE) .
	@echo "built $(BIN_DIR)/godisplacementx-cli-simd$(EXE) ($(VERSION), simd; AVX2 on amd64, scalar fallback elsewhere)"

gui:
	PATH="$(GOPATH_BIN):$$PATH" wails build

test:
	$(GO) test ./... -count=1

test-simd:
	GOEXPERIMENT=simd $(GO) test -tags simd ./... -count=1

bench:
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem -benchtime=$(BENCHTIME) $(BENCH_PKG)

bench-simd:
	GOEXPERIMENT=simd $(GO) test -tags simd -run '^$$' -bench '$(BENCH)' -benchmem -benchtime=$(BENCHTIME) $(BENCH_PKG)

# Two-build comparison: run the shared (untagged) benchmarks in the scalar and
# SIMD builds and diff them with benchstat.
bench-compare:
	@command -v benchstat >/dev/null 2>&1 || { echo "benchstat not found: go install golang.org/x/perf/cmd/benchstat@latest"; exit 1; }
	$(GO) test -run '^$$' -bench '$(BENCH)' -benchmem -count=$(COUNT) $(BENCH_PKG) | tee bench-scalar.txt
	GOEXPERIMENT=simd $(GO) test -tags simd -run '^$$' -bench '$(BENCH)' -benchmem -count=$(COUNT) $(BENCH_PKG) | tee bench-simd.txt
	benchstat bench-scalar.txt bench-simd.txt

lint:
	PATH="$(GOPATH_BIN):$$PATH" golangci-lint run ./...

sprites:
	cd tools/rasterize-sprites && npm install && npm run rasterize

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) bench-scalar.txt bench-simd.txt

# Local mirror of .github/workflows/release.yml: pure-Go CLI, 5 targets.
release:
	@mkdir -p $(DIST_DIR) build
	@set -e; \
	for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
	  os="$${t%/*}"; arch="$${t#*/}"; ext=""; [ "$$os" = windows ] && ext=".exe"; \
	  echo "==> $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS="$$os" GOARCH="$$arch" $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o "build/godisplacementx-cli$$ext" .; \
	  base="godisplacementx-cli_$(VERSION)_$${os}_$${arch}"; \
	  if [ "$$os" = windows ]; then (cd build && zip -q "../$(DIST_DIR)/$$base.zip" "godisplacementx-cli$$ext"); \
	  else tar -C build -czf "$(DIST_DIR)/$$base.tar.gz" "godisplacementx-cli$$ext"; fi; \
	  rm -f "build/godisplacementx-cli$$ext"; \
	done; \
	(cd $(DIST_DIR) && sha256sum * > SHA256SUMS.txt); ls -l $(DIST_DIR)
