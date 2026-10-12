// Package tokenizer masks dynamic tokens in log lines so that structurally
// identical lines produce identical templates regardless of per-request values.
package tokenizer

import "regexp"

// Tokenizer holds pre-compiled regular expressions applied in a fixed order.
// Order matters: earlier patterns prevent later ones from double-masking
// (e.g. UUID digits must be masked before the bare-integer rule fires).
type Tokenizer struct {
	iso8601 *regexp.Regexp // ISO 8601 / RFC 3339 timestamps
	uuid    *regexp.Regexp // xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	ipv4    *regexp.Regexp // dotted-decimal IPv4
	ipv6    *regexp.Regexp // colon-separated IPv6
	hexAddr *regexp.Regexp // 0x[0-9a-fA-F]+
	traceID *regexp.Regexp // standalone 16–32 hex chars (trace/span IDs)
	integer *regexp.Regexp // bare digit sequences
}

// New returns a ready-to-use *Tokenizer with all patterns compiled once.
func New() *Tokenizer {
	return &Tokenizer{
		// ISO 8601: 2024-01-15T10:23:11Z  2024-01-15T10:23:11.123+05:30
		iso8601: regexp.MustCompile(
			`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?`),

		// Standard UUID 8-4-4-4-12
		uuid: regexp.MustCompile(
			`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`),

		// IPv4 — each octet 0-255
		ipv4: regexp.MustCompile(
			`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`),

		// IPv6 — simplified (covers full and compressed forms)
		ipv6: regexp.MustCompile(
			`\b[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{0,4}){2,7}\b`),

		// Hex memory address: 0x followed by hex digits
		hexAddr: regexp.MustCompile(`\b0x[0-9a-fA-F]+\b`),

		// Trace/span ID: standalone 16–32 hex chars
		traceID: regexp.MustCompile(`\b[0-9a-fA-F]{16,32}\b`),

		// Bare integers
		integer: regexp.MustCompile(`\b\d+\b`),
	}
}

// Tokenize replaces all dynamic tokens in line with stable placeholders.
// The returned string is suitable for log clustering.
func (t *Tokenizer) Tokenize(line string) string {
	line = t.iso8601.ReplaceAllString(line, "<TIMESTAMP>")
	line = t.uuid.ReplaceAllString(line, "<UUID>")
	line = t.ipv4.ReplaceAllString(line, "<IP>")
	line = t.ipv6.ReplaceAllString(line, "<IP>")
	line = t.hexAddr.ReplaceAllString(line, "<ADDR>")
	line = t.traceID.ReplaceAllString(line, "<TRACEID>")
	line = t.integer.ReplaceAllString(line, "<NUM>")
	return line
}
