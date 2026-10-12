// Package pipeline wires the Reader → Tokenizer → Drain stages into a single
// concurrent processing unit.
package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/srikarmaddena/logdiff/internal/drain"
	"github.com/srikarmaddena/logdiff/internal/reader"
	"github.com/srikarmaddena/logdiff/internal/tokenizer"
)

// Config controls pipeline behaviour.
type Config struct {
	BufSize    int // reader scanner buffer in bytes (default 1 MiB)
	ChannelBuf int // inter-stage channel capacity (default 10 000)
	drain.Config
}

func (c Config) withDefaults() Config {
	if c.BufSize <= 0 {
		c.BufSize = 1 << 20
	}
	if c.ChannelBuf <= 0 {
		c.ChannelBuf = 10_000
	}
	return c
}

// Stats holds throughput metrics for a single ProcessFile run.
type Stats struct {
	LinesProcessed int64
	Duration       time.Duration
	ThroughputLPS  float64 // lines per second
	ThroughputMBPS float64 // megabytes per second
	FileSizeBytes  int64
}

// ClusterMap is a keyed collection of Drain clusters.
type ClusterMap struct {
	m map[string]*drain.LogCluster
}

// NewClusterMap returns an empty ClusterMap.
func NewClusterMap() *ClusterMap {
	return &ClusterMap{m: make(map[string]*drain.LogCluster)}
}

func (cm *ClusterMap) add(c *drain.LogCluster)  { cm.m[c.ID] = c }
func (cm *ClusterMap) Len() int                  { return len(cm.m) }

// All returns all clusters sorted by Count descending (uses Drain's Clusters which already sorts).
func (cm *ClusterMap) All() []*drain.LogCluster {
	out := make([]*drain.LogCluster, 0, len(cm.m))
	for _, c := range cm.m {
		out = append(out, c)
	}
	// Sort by count descending using a simple bubble sort
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			// Use atomic load to be consistent with Drain's Clusters() method
			if out[j].Count > out[i].Count {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// AddForTest inserts a cluster directly — used only in tests.
func (cm *ClusterMap) AddForTest(c *drain.LogCluster) { cm.m[c.ID] = c }

// ProcessFile runs the full Reader → Tokenizer → Drain pipeline on path.
// Progress is reported to stderr via a byte-based progress bar.
func ProcessFile(ctx context.Context, path string, cfg Config) (*ClusterMap, Stats, error) {
	cfg = cfg.withDefaults()

	fi, err := os.Stat(path)
	if err != nil {
		return nil, Stats{}, fmt.Errorf("pipeline: stat %q: %w", path, err)
	}
	fileSize := fi.Size()

	bar := progressbar.NewOptions64(
		fileSize,
		progressbar.OptionSetDescription(fmt.Sprintf("  %-20s", fi.Name())),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	start := time.Now()

	// Stage 1: Reader
	lineCh, errCh := reader.ReadLines(ctx, path, cfg.BufSize)

	// Stage 2: Tokenizer — mask dynamic tokens, split into token slices
	tok := tokenizer.New()
	tokenCh := make(chan []string, cfg.ChannelBuf)
	go func() {
		defer close(tokenCh)
		for line := range lineCh {
			masked := tok.Tokenize(line)
			tokens := strings.Fields(masked)
			if len(tokens) == 0 {
				continue
			}
			bar.Add(len(line) + 1) //nolint:errcheck
			select {
			case <-ctx.Done():
				return
			case tokenCh <- tokens:
			}
		}
	}()

	// Stage 3: Drain — cluster token slices
	d := drain.New(cfg.Config)
	cm := NewClusterMap()
	var lines int64
	for tokens := range tokenCh {
		c := d.Add(tokens)
		if c != nil {
			// Only add if not already in map (Drain returns existing clusters on merge)
			if _, exists := cm.m[c.ID]; !exists {
				cm.add(c)
			}
		}
		lines++
	}
	bar.Finish() //nolint:errcheck

	// Propagate reader errors
	if err := <-errCh; err != nil {
		return nil, Stats{}, err
	}

	elapsed := time.Since(start)
	secs := elapsed.Seconds()
	var lps, mbps float64
	if secs > 0 {
		lps = float64(lines) / secs
		mbps = float64(fileSize) / secs / 1e6
	}

	return cm, Stats{
		LinesProcessed: lines,
		Duration:       elapsed,
		ThroughputLPS:  lps,
		ThroughputMBPS: mbps,
		FileSizeBytes:  fileSize,
	}, nil
}
