package reader

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tempLog creates a temp file with n lines "line 1" … "line N".
func tempLog(t *testing.T, n int) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "reader_*.log")
	require.NoError(t, err)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(f, "line %d\n", i)
	}
	require.NoError(t, f.Close())
	return f.Name()
}

func drainLines(ch <-chan string) []string {
	var out []string
	for l := range ch {
		out = append(out, l)
	}
	return out
}

func TestReadLines_SmallFile(t *testing.T) {
	path := tempLog(t, 10)
	lines, errs := ReadLines(context.Background(), path, 0)
	got := drainLines(lines)
	require.NoError(t, <-errs)
	assert.Len(t, got, 10)
	assert.Equal(t, "line 1", got[0])
	assert.Equal(t, "line 10", got[9])
}

func TestReadLines_EmptyFile(t *testing.T) {
	path := tempLog(t, 0)
	lines, errs := ReadLines(context.Background(), path, 0)
	got := drainLines(lines)
	require.NoError(t, <-errs)
	assert.Len(t, got, 0)
}

func TestReadLines_FileNotFound(t *testing.T) {
	lines, errs := ReadLines(context.Background(), "/no/such/file_that_does_not_exist.log", 0)
	drainLines(lines)
	assert.Error(t, <-errs)
}

func TestReadLines_ContextCancel(t *testing.T) {
	path := tempLog(t, 100_000)
	ctx, cancel := context.WithCancel(context.Background())

	lines, errs := ReadLines(ctx, path, 0)
	// Read a few then cancel
	for i := 0; i < 5; i++ {
		<-lines
	}
	cancel()

	// Drain so the goroutine can unblock
	done := make(chan struct{})
	go func() { drainLines(lines); close(done) }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutine did not exit after context cancel")
	}
	<-errs // may be nil — that's fine
}

func TestReadLines_LargeFile(t *testing.T) {
	path := tempLog(t, 100_000)
	lines, errs := ReadLines(context.Background(), path, 0)
	got := drainLines(lines)
	require.NoError(t, <-errs)
	assert.Len(t, got, 100_000)
}

func BenchmarkReadLines(b *testing.B) {
	f, _ := os.CreateTemp(b.TempDir(), "bench_*.log")
	for i := 0; i < 100_000; i++ {
		fmt.Fprintf(f, "2024-01-15T10:23:11Z INFO svc=api req=%d status=200 latency=42ms\n", i)
	}
	f.Close()
	path := f.Name()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		lines, errs := ReadLines(context.Background(), path, 0)
		drainLines(lines)
		<-errs
	}
}
