# logdiff

[![CI](https://github.com/srikarmaddena/logdiff/actions/workflows/ci.yml/badge.svg)](https://github.com/srikarmaddena/logdiff/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/go-1.22-blue.svg)](https://go.dev)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/srikarmaddena/logdiff)](https://github.com/srikarmaddena/logdiff/releases)

**High-speed log differ for CI regression testing.**

When a backend PR changes behaviour, comparing gigabytes of container logs across staging runs is overwhelming. Existing log tools are built for production search, not for diffing two CI test runs.

logdiff ingests two raw log streams, masks dynamic tokens (timestamps, UUIDs, IPs, trace IDs), clusters events by pattern using the [Drain3](https://github.com/logpai/Drain3) algorithm, and surfaces three classes of structural regression — in seconds.

---

## The Problem

```
baseline run:  1.4 GB of logs, 1.5M lines
candidate run: 1.4 GB of logs, 1.5M lines

Q: Did this PR introduce new error patterns?
   Did any log event suddenly fire 10,000 times instead of 10?
   Did a transaction START without a matching COMPLETE?
```

Answering those questions by hand takes hours. logdiff answers them in under 45 seconds.

---

## What It Detects

| Regression Type | Description | Example |
|---|---|---|
| **New error patterns** | ERROR/FATAL/WARN lines in candidate with no match in baseline | `ERROR nil pointer dereference in handler` appears for the first time |
| **Velocity anomalies** | A log cluster fires 10× more than baseline | `WARN slow query` goes from 12 → 4,800 occurrences |
| **Missing lifecycle events** | A START/BEGIN/OPEN/INIT has no matching COMPLETE/END/CLOSE/DONE | `INFO transaction START id <*>` without a `COMPLETE` |

---

## Architecture

```
logdiff diff baseline.log candidate.log
         │
         ├─ [goroutine A] Reader ──► Tokenizer ──► Drain   (baseline ClusterMap)
         ├─ [goroutine B] Reader ──► Tokenizer ──► Drain   (candidate ClusterMap)
         │                                                          │
         └──────────────────────── DiffEngine ◄─────────────────────
                                       │
                                  RegressionDetector
                                       │
                              Reporter (terminal / JSON / HTML)
```

**Pipeline stages:**
- **Reader** — `bufio.Scanner` over a 1 MiB buffer, streaming lines over a buffered channel. Never loads the full file into memory.
- **Tokenizer** — pre-compiled regex masker. Replaces timestamps, UUIDs, IPv4/v6, hex addresses, trace IDs, and integers with stable placeholders (`<TIMESTAMP>`, `<UUID>`, `<IP>`, etc.).
- **Drain** — fixed-depth prefix-tree clustering (Drain3 algorithm). Groups structurally identical lines into templates regardless of per-request values.
- **DiffEngine** — Jaccard similarity matching between ClusterMaps. Detects new patterns, velocity deltas, and lifecycle gaps.
- **Reporter** — coloured terminal output, machine-readable JSON, and self-contained HTML report.

---

## Installation

**Go install (requires Go 1.22+)**
```bash
go install github.com/srikarmaddena/logdiff/cmd/logdiff@latest
```

**Binary download**
```bash
# macOS (Apple Silicon)
curl -sL https://github.com/srikarmaddena/logdiff/releases/latest/download/logdiff_darwin_arm64.tar.gz | tar xz
# Linux (amd64)
curl -sL https://github.com/srikarmaddena/logdiff/releases/latest/download/logdiff_linux_amd64.tar.gz | tar xz
```

**GitHub Action**
```yaml
- uses: srikarmaddena/logdiff@v1
  with:
    baseline: logs/baseline.log
    candidate: logs/candidate.log
    fail-on-regression: "true"
```

---

## Usage

### Diff two log files
```bash
logdiff diff baseline.log candidate.log
```

```
logdiff diff report — 2024-01-15T10:23:11Z
────────────────────────────────────────────────────────────
● NEW ERROR PATTERNS (1)
  [1] [ERROR] ERROR nil pointer dereference in handler
       count: 3

● VELOCITY ANOMALIES (1)
  [1] WARN slow query duration <NUM> ms
       baseline: 12  →  candidate: 4800  (400.0x increase)

● MISSING LIFECYCLE EVENTS (1)
  [1] INFO transaction START id <*>
       expected COMPLETE event — not found (count: 10)

────────────────────────────────────────────────────────────
Summary: 1 new errors  |  1 velocity anomalies  |  1 missing lifecycle events
```

### Export reports
```bash
# JSON (machine-readable, for downstream tooling)
logdiff diff baseline.log candidate.log --output report.json

# HTML (self-contained, attach to PR or upload as artifact)
logdiff diff baseline.log candidate.log --output report.html
```

### Benchmark throughput
```bash
logdiff bench testdata/HDFS_2k.log
```
```
┌─────────────────────────────────────┐
│         logdiff benchmark          │
├─────────────────────────────────────┤
│ File size:           0.17 MB        │
│ Lines:               2000           │
│ Clusters:              29           │
│ Duration:           1.2ms           │
│ Throughput:       1,666,667 lines/s │
│ Throughput:          141.7 MB/sec   │
└─────────────────────────────────────┘
```

### Render a saved report
```bash
logdiff report --input report.json
logdiff report --input report.json --output report.html
```

---

## Configuration

| Flag | Default | Description |
|---|---|---|
| `--sim-threshold` | `0.4` | Drain similarity threshold for clustering (0–1) |
| `--velocity-threshold` | `10.0` | Count ratio that triggers a velocity anomaly |
| `--output` | _(terminal)_ | Output file — `.json` or `.html` |
| `--no-color` | `false` | Disable colour output |
| `--config` | `$HOME/.logdiff.yaml` | Config file path |

---

## Benchmark Results

Measured on Apple M-series, real LogHub HDFS data:

| Dataset | Lines | File Size | Duration | Throughput |
|---|---|---|---|---|
| HDFS_2k.log (real) | 2,000 | 170 KB | ~1ms | ~1.5M lines/sec |
| Synthetic 100k | 100,000 | ~8 MB | ~80ms | ~1.2M lines/sec |
| Synthetic 1M | 1,000,000 | ~82 MB | ~850ms | ~1.2M lines/sec |

> Target: 1M lines in < 45 seconds. Actual: ~1 second. ✅

---

## Development

```bash
git clone https://github.com/srikarmaddena/logdiff
cd logdiff
make deps        # go mod tidy + verify
make build       # compile binary
make test        # unit tests + race detector + coverage gate
make lint        # golangci-lint
make bench       # Go micro-benchmarks

# Integration tests against real LogHub HDFS data
make integration
```

### Project Layout

```
cmd/logdiff/        entry point
internal/
  cli/              cobra subcommands
  reader/           streaming line reader
  tokenizer/        dynamic token masker
  drain/            Drain3 clustering algorithm
  pipeline/         goroutine pipeline
  diff/             regression detector
  report/           output renderer
pkg/version/        build-time version
integration/        real-data integration tests
testdata/           LogHub datasets (downloaded, not committed)
scripts/            dataset download scripts
examples/           sample GitHub Action workflow
.github/workflows/  ci.yml · staging.yml · release.yml
```

---

## CI/CD

| Pipeline | Trigger | What runs |
|---|---|---|
| `ci.yml` | Every PR | golangci-lint, race tests, coverage ≥ 80%, govulncheck |
| `staging.yml` | Merge to main | Integration tests on real HDFS data, perf gate |
| `release.yml` | `vX.Y.Z` tag | GoReleaser cross-platform build, GPG-signed checksums |

---

## References

- He, P. et al. "Drain: An Online Log Parsing Approach with Fixed Depth Tree." *ICWS 2017*
- LogHub dataset: [github.com/logpai/loghub](https://github.com/logpai/loghub) — real production logs from HDFS, Hadoop, Spark, and more

---

## License

Apache 2.0 — see [LICENSE](LICENSE)
