package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/srikarmaddena/logdiff/internal/diff"
	"github.com/srikarmaddena/logdiff/internal/report"
	"github.com/spf13/cobra"
)

var (
	reportInput  string
	reportFormat string
	reportOutput string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Render a saved diff report",
	Long:  `report reads a previously saved JSON diff result and renders it as terminal output or an HTML file.`,
	RunE:  runReport,
	Example: `  logdiff report --input report.json
  logdiff report --input report.json --output report.html`,
}

func runReport(cmd *cobra.Command, args []string) error {
	data, err := os.ReadFile(reportInput)
	if err != nil {
		return fmt.Errorf("reading input %q: %w", reportInput, err)
	}

	var result diff.DiffResult
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parsing report JSON: %w", err)
	}

	cfg := report.ReportConfig{
		OutputPath: reportOutput,
		NoColor:    noColor,
	}
	return report.Render(&result, cfg, os.Stdout)
}

func init() {
	rootCmd.AddCommand(reportCmd)
	reportCmd.Flags().StringVarP(&reportInput, "input", "i", "", "input JSON report file (required)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "output file (.html)")
	reportCmd.MarkFlagRequired("input") //nolint:errcheck
}
