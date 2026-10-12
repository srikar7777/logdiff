// Package report renders a DiffResult to terminal, JSON, or HTML output.
package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/srikarmaddena/logdiff/internal/diff"
)

// ReportConfig controls output behaviour.
type ReportConfig struct {
	OutputPath string // if set, write JSON or HTML depending on extension
	NoColor    bool
}

var (
	red    = color.New(color.FgRed, color.Bold)
	yellow = color.New(color.FgYellow, color.Bold)
	cyan   = color.New(color.FgCyan, color.Bold)
	green  = color.New(color.FgGreen, color.Bold)
	bold   = color.New(color.Bold)
)

// Render writes result to out (terminal) and, if OutputPath is set, to a file.
func Render(result *diff.DiffResult, cfg ReportConfig, out io.Writer) error {
	if cfg.NoColor {
		color.NoColor = true
	}
	renderTerminal(result, out)

	if cfg.OutputPath == "" {
		return nil
	}
	if strings.HasSuffix(cfg.OutputPath, ".html") {
		return renderHTML(result, cfg.OutputPath)
	}
	return renderJSON(result, cfg.OutputPath)
}

// ─── terminal renderer ────────────────────────────────────────────────────────

func renderTerminal(r *diff.DiffResult, out io.Writer) {
	fmt.Fprintf(out, "\nlogdiff report — %s\n", r.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintln(out, strings.Repeat("─", 60))

	if !r.Summary.HasRegressions {
		green.Fprintln(out, "✓  No regressions detected.") //nolint:errcheck
		return
	}

	if len(r.NewPatterns) > 0 {
		red.Fprintf(out, "\n● NEW ERROR PATTERNS (%d)\n", len(r.NewPatterns)) //nolint:errcheck
		for i, p := range r.NewPatterns {
			fmt.Fprintf(out, "  [%d] [%s] %s\n", i+1, p.Severity, p.Template)
			fmt.Fprintf(out, "       count: %d\n", p.Count)
		}
	}

	if len(r.VelocityAnomalies) > 0 {
		yellow.Fprintf(out, "\n● VELOCITY ANOMALIES (%d)\n", len(r.VelocityAnomalies)) //nolint:errcheck
		for i, v := range r.VelocityAnomalies {
			fmt.Fprintf(out, "  [%d] %s\n", i+1, v.Template)
			fmt.Fprintf(out, "       baseline: %d  →  candidate: %d  (%.1fx increase)\n",
				v.BaselineCount, v.CandidateCount, v.Ratio)
		}
	}

	if len(r.MissingLifecycles) > 0 {
		cyan.Fprintf(out, "\n● MISSING LIFECYCLE EVENTS (%d)\n", len(r.MissingLifecycles)) //nolint:errcheck
		for i, l := range r.MissingLifecycles {
			fmt.Fprintf(out, "  [%d] %s\n", i+1, l.OpenTemplate)
			fmt.Fprintf(out, "       expected %s event — not found (count: %d)\n",
				l.ExpectedClose, l.Count)
		}
	}

	fmt.Fprintln(out, strings.Repeat("─", 60))
	bold.Fprintf(out, "Summary: %d new errors  |  %d velocity anomalies  |  %d missing lifecycle events\n", //nolint:errcheck
		r.Summary.TotalNewPatterns,
		r.Summary.TotalVelocityAnomalies,
		r.Summary.TotalMissingLifecycles,
	)
}

// ─── JSON renderer ────────────────────────────────────────────────────────────

func renderJSON(r *diff.DiffResult, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("report: create JSON %q: %w", path, err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// ─── HTML renderer ────────────────────────────────────────────────────────────

const htmlTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>logdiff Report</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: 'SF Mono', 'Consolas', monospace; background: #0d1117; color: #c9d1d9; padding: 2rem; line-height: 1.6; }
  h1 { color: #58a6ff; font-size: 1.5rem; margin-bottom: .5rem; }
  .meta { color: #8b949e; font-size: .85rem; margin-bottom: 2rem; }
  h2 { font-size: 1.1rem; margin: 2rem 0 .75rem; padding: .4rem .75rem; border-radius: 4px; }
  h2.new  { background: #3d1a1a; color: #f85149; }
  h2.vel  { background: #2e2210; color: #e3b341; }
  h2.life { background: #0d2137; color: #58a6ff; }
  h2.ok   { background: #0d2e1a; color: #3fb950; }
  table { width: 100%; border-collapse: collapse; font-size: .9rem; margin-bottom: 1.5rem; }
  th { background: #161b22; color: #8b949e; font-weight: 600; text-align: left; padding: .5rem .75rem; border-bottom: 1px solid #30363d; }
  td { padding: .5rem .75rem; border-bottom: 1px solid #21262d; vertical-align: top; word-break: break-all; }
  tr:hover td { background: #161b22; }
  .sev-ERROR { color: #f85149; font-weight: bold; }
  .sev-FATAL { color: #f85149; font-weight: bold; }
  .sev-WARN  { color: #e3b341; font-weight: bold; }
  .ratio     { color: #e3b341; font-weight: bold; }
  .close-kw  { color: #58a6ff; font-weight: bold; }
  .ok-msg    { color: #3fb950; padding: 1rem; }
  footer { margin-top: 3rem; color: #484f58; font-size: .8rem; }
</style>
</head>
<body>
<h1>&#x1F50E; logdiff report</h1>
<p class="meta">Generated: {{.GeneratedAt.Format "2006-01-02T15:04:05Z07:00"}}</p>

{{if not .Summary.HasRegressions}}
  <h2 class="ok">&#x2713; No Regressions Detected</h2>
  <p class="ok-msg">The candidate log matches the baseline — no new errors, velocity anomalies, or missing lifecycle events.</p>
{{else}}

{{if .NewPatterns}}
<h2 class="new">&#x1F534; New Error Patterns ({{len .NewPatterns}})</h2>
<table>
  <tr><th>Severity</th><th>Template</th><th>Count</th></tr>
  {{range .NewPatterns}}
  <tr>
    <td><span class="sev-{{.Severity}}">{{.Severity}}</span></td>
    <td>{{.Template}}</td>
    <td>{{.Count}}</td>
  </tr>
  {{end}}
</table>
{{end}}

{{if .VelocityAnomalies}}
<h2 class="vel">&#x26A1; Velocity Anomalies ({{len .VelocityAnomalies}})</h2>
<table>
  <tr><th>Template</th><th>Baseline</th><th>Candidate</th><th>Ratio</th></tr>
  {{range .VelocityAnomalies}}
  <tr>
    <td>{{.Template}}</td>
    <td>{{.BaselineCount}}</td>
    <td>{{.CandidateCount}}</td>
    <td><span class="ratio">{{printf "%.1f" .Ratio}}x</span></td>
  </tr>
  {{end}}
</table>
{{end}}

{{if .MissingLifecycles}}
<h2 class="life">&#x1F517; Missing Lifecycle Events ({{len .MissingLifecycles}})</h2>
<table>
  <tr><th>Open Template</th><th>Expected Close</th><th>Count</th></tr>
  {{range .MissingLifecycles}}
  <tr>
    <td>{{.OpenTemplate}}</td>
    <td><span class="close-kw">{{.ExpectedClose}}</span></td>
    <td>{{.Count}}</td>
  </tr>
  {{end}}
</table>
{{end}}

{{end}}

<footer>logdiff &mdash; high-speed log differ for CI regression testing</footer>
</body>
</html>`

func renderHTML(r *diff.DiffResult, path string) error {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"not": func(b bool) bool { return !b },
	}).Parse(htmlTmpl)
	if err != nil {
		return fmt.Errorf("report: parse html template: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("report: create HTML %q: %w", path, err)
	}
	defer f.Close()
	return tmpl.Execute(f, r)
}
