package entrypoint

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatsAPI(t *testing.T) {
	s, _ := newTestServer(t, false)

	rec := do(t, s, http.MethodGet, "/api/stats", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	stats := decode[statsResponse](t, rec)
	assert.NotEmpty(t, stats.Hostname)
	assert.NotEmpty(t, stats.OS)
	assert.Greater(t, stats.Uptime, uint64(0))
	assert.Len(t, stats.LoadAvg, 3)
	assert.Greater(t, stats.CPUCores, 0)
	assert.NotEmpty(t, stats.CPU)
	assert.Greater(t, stats.Memory.Total, uint64(0))
	assert.GreaterOrEqual(t, stats.Swap.Total, uint64(0))
	assert.Len(t, stats.Processes, 50)
	assert.Len(t, stats.ProcessesByCPU, 50)

	// The server's own process is always present on the Process tab.
	require.NotNil(t, stats.SelfProcess)
	assert.Equal(t, os.Getpid(), stats.SelfProcess.PID)
	assert.NotEmpty(t, stats.SelfProcess.State)
	assert.NotEmpty(t, stats.SelfProcess.Command)
	assert.NotNil(t, stats.FocusedProcesses)
}

func TestParseProcStat(t *testing.T) {
	// comm carries spaces to exercise the closing-paren lookup.
	line := "1234 (my proc) S 1 1234 1234 0 -1 4194304 100 0 0 0 10 20 0 0 20 0 3 0 5000 4096 256"

	ps := parseProcStat(line)
	assert.Equal(t, 1234, ps.pid)
	assert.Equal(t, "my proc", ps.comm)
	assert.Equal(t, "S", ps.state)
	assert.Equal(t, 1, ps.ppid)
	assert.Equal(t, uint64(10), ps.utime)
	assert.Equal(t, uint64(20), ps.stime)
	assert.Equal(t, 3, ps.threads)
	assert.Equal(t, uint64(5000), ps.starttime)
	assert.Equal(t, uint64(4096), ps.vms)
	assert.Equal(t, uint64(256), ps.rss)
}

func TestIsFocusProcess(t *testing.T) {
	assert.True(t, isFocusProcess("opencode", ""))
	assert.True(t, isFocusProcess("codex", ""))
	assert.True(t, isFocusProcess("node", "/usr/bin/node /home/u/.opencode/bin/opencode"))
	assert.True(t, isFocusProcess("bun", "bun run codex"))
	assert.False(t, isFocusProcess("zsh", "zsh"))
	assert.False(t, isFocusProcess("mybox", "mybox serve --project demo"))
}
