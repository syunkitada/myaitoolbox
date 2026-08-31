import { useCallback, useEffect, useState } from 'react'
import {
  Activity,
  Cpu,
  HardDrive,
  MemoryStick,
  Network,
  RefreshCw,
  Server,
} from 'lucide-react'
import { Stats, StatsDisk, StatsMemory, StatsProcess, api } from '../api/client'
import { Button } from '../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { cn } from '@/lib/utils'

function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  const val = bytes / 1024 ** i
  return `${val.toFixed(val >= 100 ? 0 : 1)} ${units[i]}`
}

function formatUptime(secs: number): string {
  const d = Math.floor(secs / 86400)
  const h = Math.floor((secs % 86400) / 3600)
  const m = Math.floor((secs % 3600) / 60)
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  return `${m}m`
}

function usageColor(pct: number): string {
  if (pct >= 90) return 'bg-red-500'
  if (pct >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}

function Meter({ label, used, total, pct }: { label: string; used: string; total: string; pct: number }) {
  return (
    <div>
      <div className="flex items-baseline justify-between text-sm">
        <span className="font-medium">{label}</span>
        <span className="tabular-nums text-muted-foreground">
          {used} / {total}
        </span>
      </div>
      <div className="mt-1.5 h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={cn('h-full rounded-full transition-all', usageColor(pct))}
          style={{ width: `${Math.min(100, pct)}%` }}
        />
      </div>
      <div className="mt-0.5 text-right text-xs text-muted-foreground">{pct.toFixed(1)}%</div>
    </div>
  )
}

function StatCard({
  title,
  icon,
  value,
  sub,
  action,
}: {
  title: string
  icon: React.ReactNode
  value: string
  sub?: string
  action?: React.ReactNode
}) {
  return (
    <Card className="gap-0 py-0">
      <CardHeader className="flex flex-row items-center justify-between gap-2 py-3">
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <span className="text-muted-foreground">{icon}</span>
          {title}
        </CardTitle>
        {action}
      </CardHeader>
      <CardContent className="py-4">
        <div className="text-2xl font-semibold tabular-nums">{value}</div>
        {sub && <div className="mt-1 truncate text-xs text-muted-foreground">{sub}</div>}
      </CardContent>
    </Card>
  )
}

function DiskRow({ disk }: { disk: StatsDisk }) {
  return (
    <div key={disk.mount_point} className="grid grid-cols-[1fr_auto] items-center gap-3 py-2">
      <div className="min-w-0">
        <div className="flex items-baseline justify-between gap-2 text-sm">
          <span className="truncate font-medium">{disk.mount_point}</span>
          <span className="shrink-0 tabular-nums text-muted-foreground">
            {formatBytes(disk.used)} / {formatBytes(disk.total)}
          </span>
        </div>
        <div className="mt-1 h-2 w-full overflow-hidden rounded-full bg-muted">
          <div
            className={cn('h-full rounded-full', usageColor(disk.usage_percent))}
            style={{ width: `${Math.min(100, disk.usage_percent)}%` }}
          />
        </div>
        <div className="mt-0.5 flex items-center justify-between text-xs text-muted-foreground">
          <span className="truncate">{disk.device}</span>
          <span>{disk.usage_percent.toFixed(1)}%</span>
        </div>
      </div>
    </div>
  )
}

function MemoryCard({ title, mem }: { title: string; mem: StatsMemory }) {
  return (
    <Card className="gap-0 py-0">
      <CardHeader className="py-3">
        <CardTitle className="flex items-center gap-2 text-sm font-medium">
          <MemoryStick className="size-4 text-muted-foreground" />
          {title}
        </CardTitle>
      </CardHeader>
      <CardContent className="py-3">
        <Meter
          label={title}
          used={formatBytes(mem.used)}
          total={formatBytes(mem.total)}
          pct={mem.usage_percent}
        />
        <div className="mt-2 text-xs text-muted-foreground">
          Available: {formatBytes(mem.available)}
        </div>
      </CardContent>
    </Card>
  )
}

function ProcessTable({ processes, showCPU }: { processes: StatsProcess[]; showCPU: boolean }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b text-left text-xs text-muted-foreground">
            <th className="py-2 pr-2 font-medium">PID</th>
            <th className="py-2 pr-2 font-medium">User</th>
            {showCPU && <th className="py-2 pr-2 text-right font-medium">CPU</th>}
            <th className="py-2 pr-2 text-right font-medium">Memory</th>
            <th className="py-2 pr-2 text-right font-medium">RSS</th>
            <th className="py-2 text-left font-medium">Command</th>
          </tr>
        </thead>
        <tbody>
          {processes.map((p) => (
            <tr key={p.pid} className="border-b last:border-0">
              <td className="py-1.5 pr-2 font-mono tabular-nums">{p.pid}</td>
              <td className="py-1.5 pr-2">{p.user || '?'}</td>
              {showCPU && (
                <td className="py-1.5 pr-2 text-right tabular-nums">{p.cpu_percent.toFixed(1)}%</td>
              )}
              <td className="py-1.5 pr-2 text-right tabular-nums">{p.mem_percent.toFixed(1)}%</td>
              <td className="py-1.5 pr-2 text-right tabular-nums">{formatBytes(p.rss_bytes)}</td>
              <td className="max-w-[220px] truncate py-1.5 font-mono text-xs">{p.command}</td>
            </tr>
          ))}
          {processes.length === 0 && (
            <tr>
              <td colSpan={showCPU ? 6 : 5} className="py-3 text-muted-foreground">
                No process data available.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

export function StatsPage() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)

  const refresh = useCallback(async () => {
    setLoading(true)
    try {
      setStats(await api.getStats())
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (!autoRefresh) return
    const t = setInterval(() => void refresh(), 5000)
    return () => clearInterval(t)
  }, [autoRefresh, refresh])

  const avgCPU = stats
    ? stats.cpu.length
      ? stats.cpu.reduce((sum, c) => sum + c.usage_percent, 0) / stats.cpu.length
      : 0
    : 0

  return (
    <div className="stats-page mx-auto max-w-[1200px] space-y-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="flex items-center gap-2 text-2xl font-bold">
          <Activity className="size-6" />
          Stats
        </h1>
        <div className="flex items-center gap-2">
          <label className="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              className="size-4"
            />
            Auto-refresh (5s)
          </label>
          <Button variant="outline" size="sm" onClick={() => void refresh()} disabled={loading}>
            <RefreshCw className={cn('size-4', loading && 'animate-spin')} />
            Refresh
          </Button>
        </div>
      </div>

      {error && (
        <div className="rounded-md border border-red-200 bg-red-50 px-3.5 py-2.5 text-sm text-red-700">
          {error}
        </div>
      )}

      {stats && (
        <>
          <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <StatCard
              title="Hostname"
              icon={<Server className="size-4" />}
              value={stats.hostname || 'unknown'}
            />
            <StatCard title="Uptime" icon={<Activity className="size-4" />} value={formatUptime(stats.uptime_seconds)} />
            <StatCard
              title="Load average"
              icon={<Activity className="size-4" />}
              value={stats.load_avg.map((v) => v.toFixed(2)).join(' / ')}
              sub="1m / 5m / 15m"
            />
            <StatCard title="CPU" icon={<Cpu className="size-4" />} value={`${stats.cpu_cores} cores`} sub={stats.cpu[0]?.model} />
          </div>

          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2 lg:items-start">
            <div className="min-w-0 space-y-4">
              <Card className="gap-0 py-0">
                <CardHeader className="py-3">
                  <CardTitle className="flex items-center gap-2 text-sm font-medium">
                    <Cpu className="size-4 text-muted-foreground" />
                    CPU usage
                  </CardTitle>
                </CardHeader>
                <CardContent className="py-3">
                  <Meter label="Average" used="" total="" pct={avgCPU} />
                  <div className="mt-4 grid grid-cols-2 gap-x-6 gap-y-2">
                    {stats.cpu.map((c, i) => (
                      <div key={i} className="flex items-center gap-2 text-xs">
                        <span className="w-8 shrink-0 text-muted-foreground">#{i}</span>
                        <div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
                          <div
                            className={cn('h-full rounded-full', usageColor(c.usage_percent))}
                            style={{ width: `${Math.min(100, c.usage_percent)}%` }}
                          />
                        </div>
                        <span className="w-10 shrink-0 text-right tabular-nums">
                          {c.usage_percent.toFixed(0)}%
                        </span>
                      </div>
                    ))}
                  </div>
                </CardContent>
              </Card>

              <div className="grid grid-cols-1 gap-4">
                <MemoryCard title="Memory" mem={stats.memory} />
                <MemoryCard title="Swap" mem={stats.swap} />
                <Card className="gap-0 py-0">
                  <CardHeader className="py-3">
                    <CardTitle className="flex items-center gap-2 text-sm font-medium">
                      <HardDrive className="size-4 text-muted-foreground" />
                      Disk usage
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="grid grid-cols-1 gap-x-8 gap-y-1 py-3">
                    {stats.disks.map((d) => (
                      <DiskRow key={d.mount_point} disk={d} />
                    ))}
                    {stats.disks.length === 0 && (
                      <div className="text-sm text-muted-foreground">No physical disks found.</div>
                    )}
                  </CardContent>
                </Card>
                <Card className="gap-0 overflow-hidden py-0">
                  <CardHeader className="py-3">
                    <CardTitle className="flex items-center gap-2 text-sm font-medium">
                      <Network className="size-4 text-muted-foreground" />
                      Network
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="overflow-x-auto py-0">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b text-left text-xs text-muted-foreground">
                          <th className="py-2 font-medium">Interface</th>
                          <th className="py-2 pr-2 text-right font-medium">RX</th>
                          <th className="py-2 pr-2 text-right font-medium">TX</th>
                          <th className="py-2 text-left font-medium">State</th>
                        </tr>
                      </thead>
                      <tbody>
                        {stats.network.map((n) => (
                          <tr key={n.name} className="border-b last:border-0">
                            <td className="py-2 font-mono">{n.name}</td>
                            <td className="py-2 pr-2 text-right tabular-nums">{formatBytes(n.rx_bytes)}</td>
                            <td className="py-2 pr-2 text-right tabular-nums">{formatBytes(n.tx_bytes)}</td>
                            <td className="py-2">
                              <span
                                className={cn(
                                  'inline-flex items-center gap-1.5 text-xs',
                                  n.state === 'up' ? 'text-emerald-600' : 'text-muted-foreground',
                                )}
                              >
                                <span
                                  className={cn(
                                    'inline-block size-2 rounded-full',
                                    n.state === 'up' ? 'bg-emerald-500' : 'bg-muted-foreground/40',
                                  )}
                                />
                                {n.state}
                              </span>
                            </td>
                          </tr>
                        ))}
                        {stats.network.length === 0 && (
                          <tr>
                            <td colSpan={4} className="py-3 text-muted-foreground">
                              No network interfaces found.
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </CardContent>
                </Card>
              </div>
            </div>

            <div className="min-w-0 space-y-4 lg:sticky lg:top-16">
              <Card className="gap-0 overflow-hidden py-0">
                <CardHeader className="py-3">
                  <CardTitle className="flex items-center gap-2 text-sm font-medium">
                    <Activity className="size-4 text-muted-foreground" />
                    Processes (top by CPU)
                  </CardTitle>
                </CardHeader>
                <CardContent className="py-0">
                  <ProcessTable processes={stats.processes_by_cpu} showCPU />
                </CardContent>
              </Card>

              <Card className="gap-0 overflow-hidden py-0">
                <CardHeader className="py-3">
                  <CardTitle className="flex items-center gap-2 text-sm font-medium">
                    <Activity className="size-4 text-muted-foreground" />
                    Processes (top by memory)
                  </CardTitle>
                </CardHeader>
                <CardContent className="py-0">
                  <ProcessTable processes={stats.processes} showCPU={false} />
                </CardContent>
              </Card>
            </div>
          </div>
        </>
      )}

      {!stats && !error && (
        <div className="space-y-4">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-24 animate-pulse rounded-xl bg-muted" />
          ))}
        </div>
      )}
    </div>
  )
}
