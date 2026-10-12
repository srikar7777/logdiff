package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/srikarmaddena/logdiff/internal/diff"
	"github.com/srikarmaddena/logdiff/internal/pipeline"
	"github.com/srikarmaddena/logdiff/internal/report"
	"github.com/spf13/cobra"
)

var (
	diffOutput            string
	diffSimThreshold      float64
	diffVelocityThreshold float64
	diffChannelBuf        int
)

var diffCmd = &cobra.Command{
	Use:   "diff <baseline.log> <candidate.log>",
	Short: "Diff two log files and surface regressions",
	Long: `diff ingests two log files, masks dynamic tokens, clusters log events
by pattern, and reports structural regressions:
  • New error patterns not present in baseline
  • Velocity anomalies (cluster count ratio > threshold)
  • Missing lifecycle pairs (START without COMPLETE, etc.)`,
	Args:    cobra.ExactArgs(2),
	RunE:    runDiff,
	Example: `  logdiff diff baseline.log candidate.log
  logdiff diff baseline.log candidate.log --output report.json
  logdiff diff baseline.log candidate.log --velocity-threshold 5`,
}

func runDiff(cmd *cobra.Command, args []string) error {
	baselinePath := args[0]
	candidatePath := args[1]

	cfg := pipeline.Config{
		ChannelBuf: diffChannelBuf,
	}
	if diffSimThreshold > 0 {
		cfg.SimThreshold = diffSimThreshold
	}

	bold := color.New(color.Bold)
	bold.Fprintf(os.Stderr, "\nlogdiff diff\n") //nolint:errcheck
	fmt.Fprintf(os.Stderr, "  baseline:  %s\n", baselinePath)
	fmt.Fprintf(os.Stderr, "  candidate: %s\n\n", candidatePath)

	// Process both files concurrently
	type result struct {
		cm    *pipeline.ClusterMap
		stats pipeline.Stats
		err   error
		label string
	}

	ch := make(chan result, 2)

	go func() {
		cm, stats, err := pipeline.ProcessFile(cmd.Context(), baselinePath, cfg)
		ch <- result{cm, stats, err, "baseline"}
	}()
	go func() {
		cm, stats, err := pipeline.ProcessFile(cmd.Context(), candidatePath, cfg)
		ch <- result{cm, stats, err, "candidate"}
	}()

	var baselineCM, candidateCM *pipeline.ClusterMap
	var baselineStats, candidateStats pipeline.Stats

	for i := 0; i < 2; i++ {
		r := <-ch
		if r.err != nil {
			return fmt.Errorf("processing %s: %w", r.label, r.err)
		}
		if r.label == "baseline" {
			baselineCM, baselineStats = r.cm, r.stats
		} else {
			candidateCM, candidateStats = r.cm, r.stats
		}
	}

	fmt.Fprintf(os.Stderr, "baseline:  %d lines, %d clusters, %.0f lines/sec\n",
		baselineStats.LinesProcessed, baselineCM.Len(), baselineStats.ThroughputLPS)
	fmt.Fprintf(os.Stderr, "candidate: %d lines, %d clusters, %.0f lines/sec\n\n",
		candidateStats.LinesProcessed, candidateCM.Len(), candidateStats.ThroughputLPS)

	diffCfg := diff.DefaultDiffConfig()
	if diffVelocityThreshold > 0 {
		diffCfg.VelocityThreshold = diffVelocityThreshold
	}

	diffResult := diff.Compare(baselineCM, candidateCM, diffCfg)

	rcfg := report.ReportConfig{
		OutputPath: diffOutput,
		NoColor:    noColor,
	}
	if err := report.Render(diffResult, rcfg, os.Stdout); err != nil {
		return err
	}

	if diffResult.Summary.HasRegressions {
		os.Exit(1)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringVarP(&diffOutput, "output", "o", "", "output file (.json or .html)")
	diffCmd.Flags().Float64Var(&diffSimThreshold, "sim-threshold", 0, "Drain similarity threshold (0–1, default 0.5)")
	diffCmd.Flags().Float64Var(&diffVelocityThreshold, "velocity-threshold", 0, "velocity anomaly multiplier (default 5.0)")
	diffCmd.Flags().IntVar(&diffChannelBuf, "channel-buf", 10000, "pipeline channel buffer size")
}
