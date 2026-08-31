package entrypoint

import (
	"net/http"
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
}
