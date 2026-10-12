package report

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/srikarmaddena/logdiff/internal/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resultWithRegressions() *diff.DiffResult {
	return &diff.DiffResult{
		GeneratedAt: time.Now(),
		NewPatterns: []diff.PatternDiff{
			{Template: "ERROR nil pointer dereference", Count: 3, Severity: "ERROR"},
		},
		VelocityAnomalies: []diff.VelocityDiff{
			{Template: "WARN slow query", BaselineCount: 1, CandidateCount: 50, Ratio: 50.0},
		},
		MissingLifecycles: []diff.LifecycleDiff{
			{OpenTemplate: "INFO job START id <*>", ExpectedClose: "COMPLETE", Count: 5},
		},
		Summary: diff.Summary{
			TotalNewPatterns: 1, TotalVelocityAnomalies: 1,
			TotalMissingLifecycles: 1, HasRegressions: true,
		},
	}
}

func resultClean() *diff.DiffResult {
	return &diff.DiffResult{
		GeneratedAt: time.Now(),
		Summary:     diff.Summary{HasRegressions: false},
	}
}

func TestReporter_Terminal_NoRegression(t *testing.T) {
	buf := new(bytes.Buffer)
	err := Render(resultClean(), ReportConfig{NoColor: true}, buf)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No regressions")
}

func TestReporter_Terminal_WithRegressions(t *testing.T) {
	buf := new(bytes.Buffer)
	err := Render(resultWithRegressions(), ReportConfig{NoColor: true}, buf)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "NEW ERROR PATTERNS")
	assert.Contains(t, out, "VELOCITY ANOMALIES")
	assert.Contains(t, out, "MISSING LIFECYCLE")
	assert.Contains(t, out, "Summary:")
}

func TestReporter_JSON(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "report_*.json")
	require.NoError(t, err)
	f.Close()

	err = Render(resultWithRegressions(), ReportConfig{OutputPath: f.Name(), NoColor: true}, new(bytes.Buffer))
	require.NoError(t, err)

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)

	var parsed diff.DiffResult
	require.NoError(t, json.Unmarshal(data, &parsed))
	assert.True(t, parsed.Summary.HasRegressions)
	assert.Len(t, parsed.NewPatterns, 1)
	assert.Equal(t, "ERROR nil pointer dereference", parsed.NewPatterns[0].Template)
}

func TestReporter_HTML(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "report_*.html")
	require.NoError(t, err)
	f.Close()

	err = Render(resultWithRegressions(), ReportConfig{OutputPath: f.Name(), NoColor: true}, new(bytes.Buffer))
	require.NoError(t, err)

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	html := string(data)

	assert.True(t, strings.HasPrefix(strings.TrimSpace(html), "<!DOCTYPE html>"))
	assert.Contains(t, html, "logdiff")
	assert.Contains(t, html, "ERROR nil pointer dereference")
	assert.Contains(t, html, "Velocity Anomalies")
	assert.Contains(t, html, "Missing Lifecycle Events")
}

func TestReporter_HTML_Clean(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "report_clean_*.html")
	require.NoError(t, err)
	f.Close()

	err = Render(resultClean(), ReportConfig{OutputPath: f.Name(), NoColor: true}, new(bytes.Buffer))
	require.NoError(t, err)

	data, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Contains(t, string(data), "No Regressions")
}
