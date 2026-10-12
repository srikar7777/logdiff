package tokenizer

import "testing"

func TestTokenize(t *testing.T) {
	tok := New()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "iso8601_Z",
			input: "2024-01-15T10:23:11Z INFO server started",
			want:  "<TIMESTAMP> INFO server started",
		},
		{
			name:  "iso8601_offset",
			input: "2024-01-15T10:23:11+05:30 INFO ok",
			want:  "<TIMESTAMP> INFO ok",
		},
		{
			name:  "iso8601_fractional",
			input: "2024-01-15T10:23:11.456Z DEBUG tick",
			want:  "<TIMESTAMP> DEBUG tick",
		},
		{
			name:  "uuid",
			input: "req=550e8400-e29b-41d4-a716-446655440000 ok",
			want:  "req=<UUID> ok",
		},
		{
			name:  "ipv4",
			input: "host 192.168.1.100 port 8080",
			want:  "host <IP> port <NUM>",
		},
		{
			name:  "hex_addr",
			input: "fault at 0xdeadbeef in module",
			want:  "fault at <ADDR> in module",
		},
		{
			name:  "trace_id_16",
			input: "trace=abcdef1234567890 done",
			want:  "trace=<TRACEID> done",
		},
		{
			name:  "trace_id_32",
			input: "span abcdef1234567890abcdef1234567890 end",
			want:  "span <TRACEID> end",
		},
		{
			name:  "integer",
			input: "retried 3 times status 404",
			want:  "retried <NUM> times status <NUM>",
		},
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "no_tokens",
			input: "server started successfully",
			want:  "server started successfully",
		},
		{
			name:  "mixed_all_types",
			input: "2024-01-15T10:23:11Z ERROR peer=192.168.0.1 req=550e8400-e29b-41d4-a716-446655440000 retries=2",
			want:  "<TIMESTAMP> ERROR peer=<IP> req=<UUID> retries=<NUM>",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tok.Tokenize(tc.input)
			if got != tc.want {
				t.Errorf("\ninput: %q\nwant:  %q\ngot:   %q", tc.input, tc.want, got)
			}
		})
	}
}

func BenchmarkTokenize(b *testing.B) {
	tok := New()
	line := "2024-01-15T10:23:11Z INFO svc=api host=192.168.1.42 " +
		"req=550e8400-e29b-41d4-a716-446655440000 trace=abcdef1234567890 status=200 latency=37"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tok.Tokenize(line)
	}
}
