package pipeline

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syntheticLog creates a temp log file with n lines cycling through realistic patterns.
func syntheticLog(t *testing.T, n int) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "pipe_*.log")
	require.NoError(t, err)
	pats := []string{
		"2024-01-15T10:23:11Z INFO  svc=api  method=GET  path=/health status=200 latency=%dms",
		"2024-01-15T10:23:11Z INFO  svc=api  method=POST path=/login  status=200 latency=%dms",
		"2024-01-15T10:23:11Z WARN  svc=db   slow_query duration=%dms query=SELECT",
		"2024-01-15T10:23:11Z ERROR svc=api  connection refused host=10.0.0.%d",
		"2024-01-15T10:23:11Z DEBUG svc=cache miss key=user:%d",
		"2024-01-15T10:23:11Z INFO  svc=api  shutdown complete uptime=%ds",
		"2024-01-15T10:23:11Z WARN  svc=api  retry attempt %d of 3",
		"2024-01-15T10:23:11Z ERROR svc=db   disk full path=/data/%d",
	}
	for i := 0; i < n; i++ {
		fmt.Fprintf(f, pats[i%len(pats)]+"\n", i%1000)
	}
	require.NoError(t, f.Close())
	return f.Name()
}

func TestPipeline_EndToEnd(t *testing.T) {
	path := syntheticLog(t, 1_000)
	cm, stats, err := ProcessFile(context.Background(), path, Config{})
	require.NoError(t, err)
	assert.Equal(t, int64(1_000), stats.LinesProcessed)
	assert.Greater(t, cm.Len(), 0)
	assert.Less(t, cm.Len(), 100)
	assert.Greater(t, stats.ThroughputLPS, float64(0))
}

func TestPipeline_SameFileTwice(t *testing.T) {
	path := syntheticLog(t, 500)
	cm1, _, err := ProcessFile(context.Background(), path, Config{})
	require.NoError(t, err)
	cm2, _, err := ProcessFile(context.Background(), path, Config{})
	require.NoError(t, err)
	assert.Equal(t, cm1.Len(), cm2.Len(), "same file must produce same cluster count")
}

func TestPipeline_EmptyFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "empty_*.log")
	require.NoError(t, err)
	f.Close()
	cm, stats, err := ProcessFile(context.Background(), f.Name(), Config{})
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.LinesProcessed)
	assert.Equal(t, 0, cm.Len())
}

func TestPipeline_FileNotFound(t *testing.T) {
	_, _, err := ProcessFile(context.Background(), "/no/such/file_pipeline.log", Config{})
	assert.Error(t, err)
}

func TestPipeline_StatsPopulated(t *testing.T) {
	path := syntheticLog(t, 200)
	_, stats, err := ProcessFile(context.Background(), path, Config{})
	require.NoError(t, err)
	assert.Greater(t, stats.FileSizeBytes, int64(0))
	assert.Greater(t, stats.Duration.Nanoseconds(), int64(0))
}
