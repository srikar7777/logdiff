package cli

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/srikarmaddena/logdiff/internal/pipeline"
	"github.com/spf13/cobra"
)

var benchCmd = &cobra.Command{
	Use:     "bench <logfile>",
	Short:   "Benchmark pipeline throughput on a log file",
	Long:    `bench processes a log file through the full Reader→Tokenizer→Drain pipeline and reports throughput, memory, and cluster stats.`,
	Args:    cobra.ExactArgs(1),
	RunE:    runBench,
	Example: `  logdiff bench testdata/HDFS_2k.log`,
}

func runBench(cmd *cobra.Command, args []string) error {
	path := args[0]

	bold := color.New(color.Bold)
	bold.Fprintf(os.Stderr, "\nlogdiff bench\n") //nolint:errcheck
	fmt.Fprintf(os.Stderr, "  file: %s\n\n", path)

	cm, stats, err := pipeline.ProcessFile(cmd.Context(), path, pipeline.Config{})
	if err != nil {
		return err
	}

	fmt.Printf("┌─────────────────────────────────────┐\n")
	fmt.Printf("│         logdiff benchmark          │\n")
	fmt.Printf("├─────────────────────────────────────┤\n")
	fmt.Printf("│ File size:   %10.2f MB           │\n", float64(stats.FileSizeBytes)/1e6)
	fmt.Printf("│ Lines:       %10d             │\n", stats.LinesProcessed)
	fmt.Printf("│ Clusters:    %10d             │\n", cm.Len())
	fmt.Printf("│ Duration:    %10s             │\n", stats.Duration.Round(1e6))
	fmt.Printf("│ Throughput:  %10.0f lines/sec  │\n", stats.ThroughputLPS)
	fmt.Printf("│ Throughput:  %10.2f MB/sec     │\n", stats.ThroughputMBPS)
	fmt.Printf("└─────────────────────────────────────┘\n")

	// Print top 10 clusters
	clusters := cm.All()
	if len(clusters) > 0 {
		fmt.Printf("\nTop clusters by frequency:\n")
		limit := 10
		if len(clusters) < limit {
			limit = len(clusters)
		}
		for i, c := range clusters[:limit] {
			tmpl := c.Template
			if len(tmpl) > 12 {
				tmpl = append(tmpl[:12], "...")
			}
			fmt.Printf("  [%2d] count=%-6d  %v\n", i+1, c.Count, tmpl)
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(benchCmd)
}
