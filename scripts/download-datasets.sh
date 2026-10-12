#!/usr/bin/env bash
# Downloads real LogHub HDFS log datasets for integration tests and benchmarks.
# Source: https://github.com/logpai/loghub (LogPAI / HKUST)
# These are real production Hadoop HDFS logs — not synthetic data.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA="${SCRIPT_DIR}/../testdata"
mkdir -p "$TESTDATA"

BASE_URL="https://raw.githubusercontent.com/logpai/loghub/master/HDFS"

download() {
  local name="$1"
  local url="$2"
  local dest="${TESTDATA}/${name}"
  if [ -f "$dest" ]; then
    echo "  ✓ ${name} already present ($(wc -l < "$dest") lines)"
    return
  fi
  echo "  ↓ Downloading ${name}..."
  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget &>/dev/null; then
    wget -q "$url" -O "$dest"
  else
    echo "  ERROR: neither curl nor wget found" >&2
    exit 1
  fi
  echo "  ✓ ${name}: $(wc -l < "$dest") lines ($(du -h "$dest" | cut -f1))"
}

echo "Fetching LogHub HDFS datasets..."
echo ""

# HDFS_2k.log — 2,000 real HDFS log lines (fast CI, integration tests)
download "HDFS_2k.log" "${BASE_URL}/HDFS_2k.log"

# anomaly_label.csv — ground truth labels (for future anomaly detection validation)
download "anomaly_label.csv" "${BASE_URL}/anomaly_label.csv" || true

echo ""
echo "Datasets ready in ${TESTDATA}/"
ls -lh "$TESTDATA/"
