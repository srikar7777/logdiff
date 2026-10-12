package drain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dflt() *Drain { return New(Config{}) }

func TestDrain_SamePatternClustered(t *testing.T) {
	d := dflt()
	for i := 0; i < 10; i++ {
		d.Add([]string{"INFO", "request", fmt.Sprintf("req-%d", i), "status", "200"})
	}
	cs := d.Clusters()
	require.Len(t, cs, 1)
	assert.Equal(t, int64(10), cs[0].Count)
}

func TestDrain_DifferentPatternsSeparated(t *testing.T) {
	d := dflt()
	for _, pfx := range []string{"INFO", "WARN", "ERROR", "DEBUG"} {
		for i := 0; i < 5; i++ {
			d.Add([]string{pfx, "event", fmt.Sprintf("id-%d", i)})
		}
	}
	assert.Len(t, d.Clusters(), 4)
}

func TestDrain_VelocityCount(t *testing.T) {
	d := dflt()
	const n = 42
	for i := 0; i < n; i++ {
		d.Add([]string{"WARN", "disk", fmt.Sprintf("%d", 50+i), "percent"})
	}
	cs := d.Clusters()
	require.NotEmpty(t, cs)
	assert.Equal(t, int64(n), cs[0].Count)
}

func TestDrain_WildcardExpansion(t *testing.T) {
	d := dflt()
	d.Add([]string{"ERROR", "user", "alice", "login", "failed"})
	d.Add([]string{"ERROR", "user", "bob", "login", "failed"})
	cs := d.Clusters()
	require.Len(t, cs, 1)
	assert.Equal(t, Wildcard, cs[0].Template[2], "differing token should become wildcard")
	assert.Equal(t, "ERROR", cs[0].Template[0], "static tokens must be preserved")
	assert.Equal(t, "login", cs[0].Template[3])
}

func TestDrain_10kLines(t *testing.T) {
	d := dflt()
	pats := [][]string{
		{"INFO", "server", "started", "port", "8080"},
		{"INFO", "request", "GET", "/api/v1"},
		{"WARN", "slow", "query", "42ms"},
		{"ERROR", "connection", "refused", "host", "10.0.0.1"},
		{"DEBUG", "cache", "miss", "key", "user:1"},
		{"INFO", "shutdown", "complete"},
		{"WARN", "retry", "attempt", "1", "of", "3"},
		{"ERROR", "disk", "full", "path", "/data"},
	}
	for i := 0; i < 10_000; i++ {
		p := pats[i%len(pats)]
		tok := make([]string, len(p))
		copy(tok, p)
		tok[len(tok)-1] = fmt.Sprintf("val%d", i%100)
		d.Add(tok)
	}
	assert.Less(t, len(d.Clusters()), 50, "10k lines from %d templates should produce < 50 clusters", len(pats))
}

func TestDrain_EmptyTokens(t *testing.T) {
	d := dflt()
	result := d.Add([]string{})
	assert.Nil(t, result, "empty token slice should return nil")
	assert.Len(t, d.Clusters(), 0)
}

func BenchmarkDrain_100kLines(b *testing.B) {
	pats := [][]string{
		{"INFO", "request", "<*>", "status", "200"},
		{"WARN", "slow", "query", "<*>"},
		{"ERROR", "connection", "refused", "<*>"},
		{"DEBUG", "cache", "miss", "<*>"},
		{"INFO", "user", "login", "<*>"},
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		d := dflt()
		for i := 0; i < 100_000; i++ {
			p := pats[i%len(pats)]
			tok := make([]string, len(p))
			copy(tok, p)
			tok[len(tok)-1] = fmt.Sprintf("v%d", i)
			d.Add(tok)
		}
	}
}
