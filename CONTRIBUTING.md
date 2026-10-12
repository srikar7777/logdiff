# Contributing to logdiff

Thank you for your interest in contributing.

## Development Setup

```bash
git clone https://github.com/srikarmaddena/logdiff
cd logdiff
go mod tidy
make build
make test
```

## Running Integration Tests (Real Data)

Integration tests run against real LogHub HDFS production logs:

```bash
bash scripts/download-datasets.sh   # downloads HDFS_2k.log (~1 MB)
go test -tags integration ./integration/... -v
```

## Running Benchmarks

```bash
make bench                      # Go micro-benchmarks
make bench-hdfs                 # throughput benchmark on real HDFS data
```

## Submitting Changes

1. Fork the repo and create a feature branch off `main`
2. Write tests first — aim to keep coverage ≥ 80%
3. Ensure `make lint` and `make test` both pass locally
4. Open a PR — CI runs automatically on every push

## Code Style

- `gofmt` — enforced by CI (golangci-lint)
- Error wrapping: `fmt.Errorf("package: operation %q: %w", arg, err)`
- No `panic` outside `init()` and `cobra.CheckErr`
- All exported symbols must have a doc comment

## Project Structure

```
cmd/logdiff/        — entry point (main.go only, all logic in internal/)
internal/
  cli/              — cobra subcommands (diff, bench, report)
  reader/           — streaming bufio.Scanner line reader
  tokenizer/        — dynamic token masker (timestamps, UUIDs, IPs, etc.)
  drain/            — Drain3 log clustering algorithm
  pipeline/         — goroutine pipeline wiring all three stages
  diff/             — ClusterMap comparison and regression detection
  report/           — terminal / JSON / HTML output renderer
pkg/version/        — build-time version variables
integration/        — integration tests (real HDFS data, build tag: integration)
testdata/           — real LogHub datasets (downloaded, not committed)
scripts/            — dataset download scripts
examples/           — sample GitHub Action workflow
```
