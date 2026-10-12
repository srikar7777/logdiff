BINARY   := logdiff
MAIN     := ./cmd/logdiff
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -X github.com/srikarmaddena/logdiff/pkg/version.Version=$(VERSION) \
            -X github.com/srikarmaddena/logdiff/pkg/version.Commit=$(COMMIT) \
            -X github.com/srikarmaddena/logdiff/pkg/version.BuildDate=$(DATE)

.PHONY: build test lint bench clean deps update-golden release-dry-run download-data

## build: Compile the binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MAIN)

## test: Run all unit tests with race detector + coverage
test:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## bench: Run all Go benchmarks
bench:
	go test -run='^$$' -bench=. -benchmem ./...

## download-data: Download real LogHub HDFS datasets
download-data:
	bash scripts/download-datasets.sh

## integration: Run integration tests against real HDFS data
integration: download-data
	go test -tags integration ./integration/... -v -timeout 120s

## bench-hdfs: Benchmark against real HDFS data
bench-hdfs: build download-data
	./$(BINARY) bench testdata/HDFS_2k.log

## clean: Remove build artifacts
clean:
	rm -f $(BINARY) coverage.out report.json report.html

## deps: Tidy and verify modules
deps:
	go mod tidy
	go mod verify

## update-golden: Regenerate golden test files
update-golden:
	go test ./integration/... -tags integration -update-golden -v

## release-dry-run: Test GoReleaser config without publishing
release-dry-run:
	goreleaser release --snapshot --clean

help:
	@grep -E '^## ' Makefile | sed 's/^## /  /'
