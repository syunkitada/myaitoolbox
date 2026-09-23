import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { Activity, Bot, Box, Boxes, Cpu, GitBranch, HardDrive, MemoryStick, SquareKanban } from 'lucide-react'
import { api, Meta, Stats } from '../api/client'
import { clearProject, dirName, encodePath, projectUrlFor, setProject } from '../utils/routes'
import type { HerdrAgent, HerdrOverview, ProjectGitStatus } from '../api/client'
import { filePathForAgent } from '../utils/herdr-file-agent'
import { statusDotClass } from './herdr-status'
import {
  Sidebar,
  SidebarHeader,
  SidebarContent,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { Separator } from '@/components/ui/separator'

interface SidebarProps {
  meta: Meta | null
  project: string
  herdr: HerdrOverview | null
  gitStatus: Record<string, ProjectGitStatus>
}

const STATS_REFRESH_INTERVAL_MS = 5000

function clampUsage(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

function usageBarColor(value: number): string {
  if (value >= 90) return 'bg-red-500'
  if (value >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}

function formatSidebarBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const unit = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)))
  const value = bytes / 1024 ** unit
  return `${value.toFixed(value >= 100 ? 0 : 1)} ${units[unit]}`
}

function useSidebarStats() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const requestSeq = useRef(0)

  const refresh = useCallback(async () => {
    const seq = ++requestSeq.current
    try {
      const next = await api.getStats()
      if (seq !== requestSeq.current) return
      setStats(next)
      setError(false)
    } catch {
      if (seq !== requestSeq.current) return
      setError(true)
    } finally {
      if (seq === requestSeq.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
    const interval = window.setInterval(() => {
      if (!document.hidden) void refresh()
    }, STATS_REFRESH_INTERVAL_MS)
    const onVisible = () => {
      if (!document.hidden) void refresh()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      requestSeq.current += 1
      window.clearInterval(interval)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refresh])

  return { stats, loading, error }
}

function SidebarMetric({
  icon,
  label,
  percent,
  detail,
}: {
  icon: ReactNode
  label: string
  percent: number | null
  detail?: string
}) {
  const available = percent !== null && Number.isFinite(percent)
  const safePercent = available ? clampUsage(percent) : 0
  const value = available ? `${safePercent.toFixed(0)}%` : '—'
  const description = `${label}: ${value}${detail ? ` (${detail})` : ''}`

  return (
    <div
      className="flex min-w-0 items-center gap-1 text-[11px]"
      data-testid={`sidebar-stat-${label.toLowerCase().replaceAll(' ', '-')}`}
      title={description}
      aria-label={description}
    >
      <span className="shrink-0 text-sidebar-foreground/75">{icon}</span>
      <span className="truncate text-sidebar-foreground/75">{label}</span>
      <span
        className={`size-1.5 shrink-0 rounded-full ${usageBarColor(safePercent)}`}
        aria-hidden="true"
      />
      <span className="shrink-0 font-medium tabular-nums">{value}</span>
    </div>
  )
}

function SidebarStatsSummary({ stats, loading, error }: ReturnType<typeof useSidebarStats>) {
  if (!stats && loading) {
    return (
      <div className="sidebar-stats-summary flex gap-3 px-2 pt-1 pb-2" data-testid="sidebar-stats-summary">
        {[0, 1, 2].map((item) => (
          <div key={item} className="h-3 min-w-0 flex-1 animate-pulse rounded bg-sidebar-border/70" aria-hidden="true" />
        ))}
      </div>
    )
  }

  if (!stats) {
    return (
      <div className="sidebar-stats-summary px-2 pt-1 pb-2 text-[11px] text-sidebar-foreground/55" data-testid="sidebar-stats-summary">
        System stats unavailable.
      </div>
    )
  }

  const cpu = stats.cpu.length
    ? stats.cpu.reduce((sum, core) => sum + core.usage_percent, 0) / stats.cpu.length
    : null
  const disk = stats.disks.find((item) => item.mount_point === '/') ?? stats.disks[0]

  return (
    <div
      className="sidebar-stats-summary flex items-center justify-between gap-3 px-2 pt-1 pb-2"
      data-testid="sidebar-stats-summary"
      title="Updates every 5 seconds"
    >
      <SidebarMetric icon={<Cpu className="size-3 shrink-0" />} label="CPU" percent={cpu} />
      <SidebarMetric
        icon={<MemoryStick className="size-3 shrink-0" />}
        label="RAM"
        percent={stats.memory.usage_percent}
        detail={`${formatSidebarBytes(stats.memory.used)} / ${formatSidebarBytes(stats.memory.total)}`}
      />
      <SidebarMetric
        icon={<HardDrive className="size-3 shrink-0" />}
        label="Disk"
        percent={disk?.usage_percent ?? null}
        detail={
          disk
            ? `${disk.mount_point}: ${formatSidebarBytes(disk.used)} / ${formatSidebarBytes(disk.total)}`
            : 'No disk data'
        }
      />
      {error && (
        <span className="shrink-0 text-[10px] text-sidebar-foreground/50" title="Last update failed">
          !
        </span>
      )}
    </div>
  )
}

export function AppSidebar({ meta, project, herdr, gitStatus }: SidebarProps) {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { setOpenMobile, state } = useSidebar()
  const collapsed = state === 'collapsed'
  const sidebarStats = useSidebarStats()

  const handleNav = () => {
    setOpenMobile(false)
  }

  const workspaceStatus = (name: string): string | null =>
    herdr?.workspaces.find((w) => w.label === name)?.agent_status ?? null

  // workspaceProject maps a herdr workspace to a known project label, so an
  // agent can be opened in the Herdr tab of the project its workspace belongs
  // to rather than always the currently-active project.
  const workspaceProject = (workspaceId?: string): string | null => {
    if (!workspaceId) return null
    const ws = herdr?.workspaces.find((w) => w.workspace_id === workspaceId)
    const label = ws?.label
    return label && meta?.projects?.includes(label) ? label : null
  }

  // openAgent opens the agent's linked task file in the Files tab when the
  // agent belongs to a task directory (tasks/<dir>/...); otherwise it opens
  // the Herdr tab of the agent's own workspace project (when it maps to a
  // known project) or falls back to the current/default project, with the
  // agent's operation panel pre-opened.
  const openAgent = async (agent: HerdrAgent, fallbackProject?: string) => {
    const agentProject =
      workspaceProject(agent.workspace_id) ||
      (fallbackProject && meta?.projects?.includes(fallbackProject) ? fallbackProject : null) ||
      project ||
      meta?.default_project ||
      meta?.projects?.[0] ||
      ''
    if (!agentProject) return
    handleNav()
    // File agents carry a custom name derived from their task directory; only
    // those can be linked back to a file. Generic agents skip the file lookup.
    if (agent.custom_name) {
      try {
        const entries = await api.listFiles({ project: agentProject })
        const filePath = filePathForAgent(
          entries.filter((e) => e.kind === 'file'),
          agent.name,
        )
        if (filePath) {
          navigate(projectUrlFor(agentProject, `/dashboard/files/${encodePath(filePath)}`))
          return
        }
      } catch {
        // Listing files failed: fall back to the Herdr tab below.
      }
    }
    navigate(`${projectUrlFor(agentProject, '/herdr')}?agent=${encodeURIComponent(agent.pane_id)}`)
  }

  return (
    <Sidebar collapsible="icon" className="border-r">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              size="lg"
              onClick={() => clearProject(navigate)}
              aria-label="Go to top"
              tooltip="mybox"
              className="cursor-pointer"
            >
              <div className="flex aspect-square size-8 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                <Box className="size-4" />
              </div>
              <span className="truncate text-base font-semibold">mybox</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>

      <SidebarContent className="gap-1">
        <SidebarMenu className="sidebar-nav mt-1">
          <SidebarMenuItem>
            <SidebarMenuButton asChild tooltip="Workspaces" isActive={pathname === '/projects'}>
              <NavLink to="/projects" end onClick={handleNav}>
                <Boxes />
                <span>Workspaces</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem>
            <SidebarMenuButton asChild tooltip="Board" isActive={pathname === '/board'}>
              <NavLink to="/board" end onClick={handleNav}>
                <SquareKanban />
                <span>Board</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem>
            <SidebarMenuButton asChild tooltip="Stats" isActive={pathname === '/stats'}>
              <NavLink to="/stats" end onClick={handleNav}>
                <Activity />
                <span>Stats</span>
              </NavLink>
            </SidebarMenuButton>
            {!collapsed && <SidebarStatsSummary {...sidebarStats} />}
          </SidebarMenuItem>
        </SidebarMenu>

        <Separator className="mx-2 my-1 group-data-[collapsible=icon]:mx-3" />

        <SidebarMenu className="sidebar-projects mt-1 pb-2">
          {(meta?.projects ?? []).map((p) => {
            const status = workspaceStatus(p)
            const gs = gitStatus[p]
            return (
              <SidebarMenuItem key={p}>
                <SidebarMenuButton
                  onClick={() => {
                    handleNav()
                    if (p !== project) setProject(p, navigate)
                  }}
                  isActive={project === p}
                  tooltip={status ? `${p} (${status})` : p}
                  className="cursor-pointer"
                >
                  <Box />
                  <span className="truncate">{p}</span>
                  <span className="ml-auto flex shrink-0 items-center gap-1">
                    {gs?.dirty && (
                      <span
                        className="git-status flex items-center text-amber-500"
                        role="img"
                        aria-label={`git: ${gs.staged} staged, ${gs.modified} modified, ${gs.untracked} untracked`}
                        title={`git: ${gs.staged} staged, ${gs.modified} modified, ${gs.untracked} untracked`}
                      >
                        <GitBranch className="size-3.5" />
                      </span>
                    )}
                    {status && (
                      <span
                        className={`herdr-workspace-status inline-block size-2 shrink-0 rounded-full ${statusDotClass(status)} ${status === 'working' ? 'animate-pulse' : ''}`}
                        role="img"
                        aria-label={`workspace status ${status}`}
                      />
                    )}
                  </span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
          {!meta || meta.projects.length === 0 ? (
            <p className="px-2 text-xs text-sidebar-foreground/60 group-data-[collapsible=icon]:hidden">
              No workspaces yet.
            </p>
          ) : null}
        </SidebarMenu>

        <Separator className="mx-2 my-1 group-data-[collapsible=icon]:mx-3" />

        <SidebarMenu className="sidebar-agents mt-1 pb-2">
          {!collapsed && (
            <li
              className="sidebar-agents-label flex items-center gap-1.5 px-2 text-[11px] font-semibold tracking-wider text-sidebar-foreground/60 uppercase"
              data-testid="sidebar-agents-label"
            >
              <Bot className="size-3.5" />
              Agents
              {herdr?.available === false && (
                <span className="ml-auto text-[10px] normal-case opacity-60">offline</span>
              )}
            </li>
          )}
          {(herdr?.agents ?? []).map((a) => {
            const dir = dirName(a.cwd)
            return (
              <SidebarMenuItem key={a.pane_id}>
                <SidebarMenuButton
                  onClick={() => void openAgent(a)}
                  tooltip={[a.status, a.cwd, a.title].filter(Boolean).join(' · ')}
                  className="sidebar-agent-row cursor-pointer"
                  data-testid={`sidebar-agent-${a.pane_id}`}
                >
                  <Bot />
                  <span className="flex min-w-0 flex-1 items-center gap-1.5">
                    <span className="truncate text-[13px] font-medium">{dir}: {a.name || a.title}</span>
                    <span
                      className={`ml-auto inline-block size-2 shrink-0 rounded-full ${statusDotClass(a.status)} ${a.status === 'working' ? 'animate-pulse' : ''}`}
                      role="img"
                      aria-label={`agent status ${a.status}`}
                    />
                  </span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
          {herdr && herdr.available && herdr.agents.length === 0 && !collapsed ? (
            <p className="px-2 text-xs text-sidebar-foreground/60">No agents running.</p>
          ) : null}
        </SidebarMenu>
      </SidebarContent>

      <SidebarRail />
    </Sidebar>
  )
}
