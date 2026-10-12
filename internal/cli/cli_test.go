package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exec runs the root command with args and returns combined output + error.
func exec(args ...string) (string, error) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(args)
	_, err := rootCmd.ExecuteC()
	return buf.String(), err
}

func TestHelp(t *testing.T) {
	out, err := exec("--help")
	require.NoError(t, err)
	assert.Contains(t, out, "logdiff")
	assert.Contains(t, out, "diff")
	assert.Contains(t, out, "bench")
	assert.Contains(t, out, "report")
}

func TestVersion(t *testing.T) {
	out, err := exec("--version")
	require.NoError(t, err)
	assert.Contains(t, out, "logdiff")
}

func TestDiff_RequiresTwoArgs(t *testing.T) {
	_, err := exec("diff", "only.log")
	assert.Error(t, err)
}

func TestDiff_TooManyArgs(t *testing.T) {
	_, err := exec("diff", "a.log", "b.log", "c.log")
	assert.Error(t, err)
}

func TestBench_RequiresOneArg(t *testing.T) {
	_, err := exec("bench")
	assert.Error(t, err)
}

func TestBench_TooManyArgs(t *testing.T) {
	_, err := exec("bench", "a.log", "b.log")
	assert.Error(t, err)
}

func TestReport_RequiresInputFlag(t *testing.T) {
	_, err := exec("report")
	assert.Error(t, err)
}

func TestUnknownCommand(t *testing.T) {
	_, err := exec("unknowncmd")
	assert.Error(t, err)
}
