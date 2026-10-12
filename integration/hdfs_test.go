//go:build integration

// Package integration runs end-to-end tests against real LogHub HDFS data.
// Run with: go test -tags integration ./integration/... -v
// Requires: bash scripts/download-datasets.sh to have been run first.
package integration

import (
	"bufio"
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/srikarmaddena/logdiff/internal/diff"
	"github.com/srikarmaddena/logdiff/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate golden files")

func hdfsPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "testdata", "HDFS_2k.log")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("HDFS_2k.log not found — run: bash scripts/download-datasets.sh")
	}
	return path
}

// TestHDFS_LineCount verifies that the pipeline reads every line in the file.
func TestHDFS_LineCount(t *testing.T) {
	path := hdfsPath(t)

	// Count lines via standard scanner
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var expected int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		expected++
	}
	require.NoError(t, sc.Err())

	_, stats, err := pipeline.ProcessFile(context.Background(), path, pipeline.Config{})
	require.NoError(t, err)
	assert.Equal(t, expected, stats.LinesProcessed,
		"pipeline must read every line in the real HDFS log")
	t.Logf("HDFS_2k: %d lines read in %s (%.0f lines/sec, %.2f MB/sec)",
		stats.LinesProcessed, stats.Duration.Round(1e6),
		stats.ThroughputLPS, stats.ThroughputMBPS)
}

// TestHDFS_ClusterCount verifies that real HDFS logs produce a sane cluster count.
// Real HDFS_2k.log has ~29 distinct log templates per the LogPAI paper.
func TestHDFS_ClusterCount(t *testing.T) {
	path := hdfsPath(t)
	cm, stats, err := pipeline.ProcessFile(context.Background(), path, pipeline.Config{})
	require.NoError(t, err)

	t.Logf("Clusters: %d from %d real HDFS lines", cm.Len(), stats.LinesProcessed)

	// Real HDFS logs have between 20 and 50 distinct templates
	assert.GreaterOrEqual(t, cm.Len(), 5, "should find at least 5 clusters in real HDFS data")
	assert.LessOrEqual(t, cm.Len(), 200, "real HDFS data should not produce > 200 clusters")
}

// TestHDFS_DiffSameFile verifies that diffing a file against itself yields zero regressions.
func TestHDFS_DiffSameFile(t *testing.T) {
	path := hdfsPath(t)
	ctx := context.Background()

	baseline, _, err := pipeline.ProcessFile(ctx, path, pipeline.Config{})
	require.NoError(t, err)

	candidate, _, err := pipeline.ProcessFile(ctx, path, pipeline.Config{})
	require.NoError(t, err)

	result := diff.Compare(baseline, candidate, diff.DefaultDiffConfig())
	assert.False(t, result.Summary.HasRegressions,
		"diffing a file against itself must produce zero regressions")
	assert.Equal(t, 0, result.Summary.TotalNewPatterns)
	assert.Equal(t, 0, result.Summary.TotalVelocityAnomalies)
}

// TestHDFS_Throughput is a performance gate: must process HDFS_2k in < 5 seconds.
func TestHDFS_Throughput(t *testing.T) {
	path := hdfsPath(t)
	_, stats, err := pipeline.ProcessFile(context.Background(), path, pipeline.Config{})
	require.NoError(t, err)

	t.Logf("Throughput: %.0f lines/sec, %.2f MB/sec, duration: %s",
		stats.ThroughputLPS, stats.ThroughputMBPS, stats.Duration.Round(1e6))

	assert.Greater(t, stats.ThroughputLPS, float64(1000),
		"must process at least 1,000 lines/sec on real HDFS data")

	// Print top clusters for visibility in CI logs
	if testing.Verbose() || *updateGolden {
		_, stats2, _ := pipeline.ProcessFile(context.Background(), path, pipeline.Config{})
		t.Logf("Stats: %+v", stats2)
	}
}

// TestHDFS_TopClusters logs the top 10 clusters from real HDFS data for inspection.
func TestHDFS_TopClusters(t *testing.T) {
	path := hdfsPath(t)
	cm, _, err := pipeline.ProcessFile(context.Background(), path, pipeline.Config{})
	require.NoError(t, err)

	clusters := cm.All()
	limit := 10
	if len(clusters) < limit {
		limit = len(clusters)
	}
	t.Logf("Top %d clusters from real HDFS_2k.log:", limit)
	for i, c := range clusters[:limit] {
		t.Logf("  [%2d] count=%-5d  %v", i+1, c.Count, c.Template)
	}
}
