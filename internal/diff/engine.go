// Package diff compares two ClusterMaps and detects structural regressions.
package diff

import (
	"strings"
	"time"

	"github.com/srikarmaddena/logdiff/internal/drain"
	"github.com/srikarmaddena/logdiff/internal/pipeline"
)

// DiffConfig controls regression detection thresholds.
type DiffConfig struct {
	// VelocityThreshold is the count ratio above which a cluster is flagged
	// as a velocity anomaly (default 10.0 = 10× increase).
	VelocityThreshold float64
	// SimMatchThreshold is the Jaccard similarity above which two templates
	// are considered the "same" pattern (default 0.8).
	SimMatchThreshold float64
	// ErrorLevels are the log-level tokens that trigger new-pattern alerts.
	ErrorLevels []string
}

// DefaultDiffConfig returns production-safe defaults.
func DefaultDiffConfig() DiffConfig {
	return DiffConfig{
		VelocityThreshold: 5.0,
		SimMatchThreshold: 0.6,
		ErrorLevels:       []string{"ERROR", "FATAL", "WARN"},
	}
}

// PatternDiff is a log pattern present in candidate but absent from baseline.
type PatternDiff struct {
	Template string `json:"template"`
	Count    int64  `json:"count"`
	Severity string `json:"severity"`
}

// VelocityDiff is a pattern whose occurrence rate grew beyond the threshold.
type VelocityDiff struct {
	Template       string  `json:"template"`
	BaselineCount  int64   `json:"baseline_count"`
	CandidateCount int64   `json:"candidate_count"`
	Ratio          float64 `json:"ratio"`
}

// LifecycleDiff is an open-lifecycle event with no matching close event.
type LifecycleDiff struct {
	OpenTemplate  string `json:"open_template"`
	ExpectedClose string `json:"expected_close"`
	Count         int64  `json:"count"`
}

// Summary aggregates counts across all regression types.
type Summary struct {
	TotalNewPatterns       int  `json:"total_new_patterns"`
	TotalVelocityAnomalies int  `json:"total_velocity_anomalies"`
	TotalMissingLifecycles int  `json:"total_missing_lifecycles"`
	HasRegressions         bool `json:"has_regressions"`
}

// DiffResult is the full output of a Compare run.
type DiffResult struct {
	NewPatterns       []PatternDiff  `json:"new_patterns"`
	VelocityAnomalies []VelocityDiff `json:"velocity_anomalies"`
	MissingLifecycles []LifecycleDiff `json:"missing_lifecycles"`
	Summary           Summary        `json:"summary"`
	GeneratedAt       time.Time      `json:"generated_at"`
}

// Compare compares baseline and candidate ClusterMaps and returns a DiffResult.
func Compare(baseline, candidate *pipeline.ClusterMap, cfg DiffConfig) *DiffResult {
	result := &DiffResult{GeneratedAt: time.Now()}

	baseClusters := baseline.All()
	candClusters := candidate.All()

	// ── 1. New patterns & velocity anomalies ─────────────────────────────────
	for _, cand := range candClusters {
		bestBase, bestScore := findBestMatch(cand, baseClusters)

		if bestBase == nil || bestScore < cfg.SimMatchThreshold {
			// No matching baseline cluster — genuinely new pattern
			if isErrorLevel(cand.Template, cfg.ErrorLevels) {
				result.NewPatterns = append(result.NewPatterns, PatternDiff{
					Template: templateStr(cand.Template),
					Count:    cand.Count,
					Severity: firstErrorLevel(cand.Template, cfg.ErrorLevels),
				})
			}
		} else {
			// Matched baseline cluster — check velocity
			if bestBase.Count > 0 {
				ratio := float64(cand.Count) / float64(bestBase.Count)
				if ratio > cfg.VelocityThreshold {
					result.VelocityAnomalies = append(result.VelocityAnomalies, VelocityDiff{
						Template:       templateStr(cand.Template),
						BaselineCount:  bestBase.Count,
						CandidateCount: cand.Count,
						Ratio:          ratio,
					})
				}
			} else if isErrorLevel(cand.Template, cfg.ErrorLevels) && cand.Count > 0 {
				// Pattern exists in baseline with 0 count but has errors in candidate
				result.NewPatterns = append(result.NewPatterns, PatternDiff{
					Template: templateStr(cand.Template),
					Count:    cand.Count,
					Severity: firstErrorLevel(cand.Template, cfg.ErrorLevels),
				})
			}
		}
	}

	// ── 2. Missing lifecycle pairs ────────────────────────────────────────────
	openKws  := []string{"START", "BEGIN", "OPEN", "INIT"}
	closeKws := []string{"COMPLETE", "END", "CLOSE", "DONE"}

	for _, cand := range candClusters {
		tmplUp := strings.ToUpper(templateStr(cand.Template))
		openIdx := -1
		for i, kw := range openKws {
			// Use word boundary check: must be standalone word, not substring
			if containsWord(tmplUp, kw) {
				openIdx = i
				break
			}
		}
		if openIdx < 0 {
			continue
		}
		expectedKw := closeKws[openIdx]
		found := false
		for _, other := range candClusters {
			otherUp := strings.ToUpper(templateStr(other.Template))
			if containsWord(otherUp, expectedKw) {
				if jaccard(cand.Template, other.Template) > 0.1 {
					found = true
					break
				}
			}
		}
		if !found {
			result.MissingLifecycles = append(result.MissingLifecycles, LifecycleDiff{
				OpenTemplate:  templateStr(cand.Template),
				ExpectedClose: expectedKw,
				Count:         cand.Count,
			})
		}
	}

	result.Summary = Summary{
		TotalNewPatterns:       len(result.NewPatterns),
		TotalVelocityAnomalies: len(result.VelocityAnomalies),
		TotalMissingLifecycles: len(result.MissingLifecycles),
	}
	result.Summary.HasRegressions = result.Summary.TotalNewPatterns > 0 ||
		result.Summary.TotalVelocityAnomalies > 0 ||
		result.Summary.TotalMissingLifecycles > 0

	return result
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func findBestMatch(target *drain.LogCluster, candidates []*drain.LogCluster) (*drain.LogCluster, float64) {
	var best *drain.LogCluster
	var bestScore float64
	for _, c := range candidates {
		if s := jaccard(target.Template, c.Template); s > bestScore {
			bestScore, best = s, c
		}
	}
	return best, bestScore
}

// jaccard computes intersection/union over the non-wildcard token sets.
func jaccard(a, b []string) float64 {
	sa := tokenSet(a)
	sb := tokenSet(b)
	var inter, union int
	for k := range sa {
		union++
		if sb[k] {
			inter++
		}
	}
	for k := range sb {
		if !sa[k] {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenSet(tmpl []string) map[string]bool {
	s := make(map[string]bool, len(tmpl))
	for _, t := range tmpl {
		if t != drain.Wildcard && t != "" {
			s[t] = true
		}
	}
	return s
}

func isErrorLevel(tmpl []string, levels []string) bool {
	return firstErrorLevel(tmpl, levels) != ""
}

// firstErrorLevel scans all tokens in the template for a severity keyword.
// HDFS logs have format: <NUM> <NUM> <NUM> ERROR ... so we can't just check tmpl[0].
func firstErrorLevel(tmpl []string, levels []string) string {
	for _, tok := range tmpl {
		up := strings.ToUpper(tok)
		for _, l := range levels {
			if up == l {
				return l
			}
		}
	}
	return ""
}

func templateStr(tmpl []string) string {
	return strings.Join(tmpl, " ")
}

// containsWord checks if a word exists as a standalone token in a string.
// This prevents false positives like "STARTED" matching "START".
func containsWord(s, word string) bool {
	words := strings.Fields(s)
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}
