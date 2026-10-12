package diff

import (
	"testing"

	"github.com/srikarmaddena/logdiff/internal/drain"
	"github.com/srikarmaddena/logdiff/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeMap(clusters []*drain.LogCluster) *pipeline.ClusterMap {
	cm := pipeline.NewClusterMap()
	for _, c := range clusters {
		cm.AddForTest(c)
	}
	return cm
}

func cl(id string, tmpl []string, count int64) *drain.LogCluster {
	return &drain.LogCluster{ID: id, Template: tmpl, Count: count}
}

func TestDiff_NoRegression(t *testing.T) {
	base := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "server", "started"}, 1),
		cl("c2", []string{"INFO", "request", "ok"}, 100),
	})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "server", "started"}, 1),
		cl("c2", []string{"INFO", "request", "ok"}, 100),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	t.Logf("Summary: NewPatterns=%d, Velocity=%d, Lifecycle=%d, HasRegressions=%v", 
		result.Summary.TotalNewPatterns, result.Summary.TotalVelocityAnomalies, 
		result.Summary.TotalMissingLifecycles, result.Summary.HasRegressions)
	if len(result.MissingLifecycles) > 0 {
		t.Logf("  MissingLifecycle[0]: %+v", result.MissingLifecycles[0])
	}
	assert.False(t, result.Summary.HasRegressions)
	assert.Equal(t, 0, result.Summary.TotalNewPatterns)
	assert.Equal(t, 0, result.Summary.TotalVelocityAnomalies)
	assert.Equal(t, 0, result.Summary.TotalMissingLifecycles)
}

func TestDiff_NewErrorPattern(t *testing.T) {
	base := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "server", "started"}, 1),
	})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "server", "started"}, 1),
		cl("c2", []string{"ERROR", "nil", "pointer", "dereference"}, 5),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	require.Equal(t, 1, result.Summary.TotalNewPatterns)
	assert.Contains(t, result.NewPatterns[0].Template, "ERROR")
	assert.Equal(t, int64(5), result.NewPatterns[0].Count)
	assert.Equal(t, "ERROR", result.NewPatterns[0].Severity)
}

func TestDiff_NewWarnPattern(t *testing.T) {
	base := makeMap([]*drain.LogCluster{})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"WARN", "connection", "timeout", "host", "<*>"}, 3),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	require.Equal(t, 1, result.Summary.TotalNewPatterns)
	assert.Equal(t, "WARN", result.NewPatterns[0].Severity)
}

func TestDiff_InfoPatternNotFlagged(t *testing.T) {
	// INFO-level new patterns should NOT be flagged
	base := makeMap([]*drain.LogCluster{})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "new", "feature", "enabled"}, 1),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	assert.Equal(t, 0, result.Summary.TotalNewPatterns)
}

func TestDiff_VelocityAnomaly(t *testing.T) {
	base := makeMap([]*drain.LogCluster{
		cl("c1", []string{"WARN", "disk", "usage", "high"}, 1),
	})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"WARN", "disk", "usage", "high"}, 100),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	require.Equal(t, 1, result.Summary.TotalVelocityAnomalies)
	assert.InDelta(t, 100.0, result.VelocityAnomalies[0].Ratio, 0.01)
	assert.Equal(t, int64(1), result.VelocityAnomalies[0].BaselineCount)
	assert.Equal(t, int64(100), result.VelocityAnomalies[0].CandidateCount)
}

func TestDiff_VelocityBelowThreshold(t *testing.T) {
	// 5x increase, threshold is 10x — should NOT be flagged
	base := makeMap([]*drain.LogCluster{
		cl("c1", []string{"WARN", "slow", "query"}, 10),
	})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"WARN", "slow", "query"}, 50),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	assert.Equal(t, 0, result.Summary.TotalVelocityAnomalies)
}

func TestDiff_MissingLifecycle_Start(t *testing.T) {
	base := makeMap([]*drain.LogCluster{})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "transaction", "START", "id", "<*>"}, 10),
		// No COMPLETE cluster
	})
	result := Compare(base, cand, DefaultDiffConfig())
	require.Equal(t, 1, result.Summary.TotalMissingLifecycles)
	assert.Contains(t, result.MissingLifecycles[0].ExpectedClose, "COMPLETE")
}

func TestDiff_MissingLifecycle_Begin(t *testing.T) {
	base := makeMap([]*drain.LogCluster{})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "job", "BEGIN", "<*>"}, 5),
		// No END cluster
	})
	result := Compare(base, cand, DefaultDiffConfig())
	require.Equal(t, 1, result.Summary.TotalMissingLifecycles)
	assert.Equal(t, "END", result.MissingLifecycles[0].ExpectedClose)
}

func TestDiff_LifecyclePaired(t *testing.T) {
	// Both START and COMPLETE present — should not flag
	base := makeMap([]*drain.LogCluster{})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "job", "START", "<*>"}, 10),
		cl("c2", []string{"INFO", "job", "COMPLETE", "<*>"}, 10),
	})
	result := Compare(base, cand, DefaultDiffConfig())
	assert.Equal(t, 0, result.Summary.TotalMissingLifecycles)
}

func TestDiff_AllThreeRegressions(t *testing.T) {
	base := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "server", "started"}, 1),
		cl("c2", []string{"WARN", "slow", "query"}, 1),
	})
	cand := makeMap([]*drain.LogCluster{
		cl("c1", []string{"INFO", "server", "started"}, 1),
		cl("c2", []string{"WARN", "slow", "query"}, 50),          // velocity
		cl("c3", []string{"ERROR", "panic", "recovered"}, 3),     // new pattern
		cl("c4", []string{"INFO", "job", "BEGIN", "id", "<*>"}, 5), // lifecycle
	})
	result := Compare(base, cand, DefaultDiffConfig())
	assert.True(t, result.Summary.HasRegressions)
	assert.Equal(t, 1, result.Summary.TotalNewPatterns)
	assert.Equal(t, 1, result.Summary.TotalVelocityAnomalies)
	assert.Equal(t, 1, result.Summary.TotalMissingLifecycles)
}
