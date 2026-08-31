import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { Bot, Box, Boxes, GitBranch, SquareKanban, Activity } from 'lucide-react'
import { Meta } from '../api/client'
import { clearProject, dirName, projectUrlFor, setProject } from '../utils/routes'
import type { HerdrOverview, ProjectGitStatus } from '../api/client'
import { statusDotClass } from './herdr-status'
import {
  Sidebar,
  SidebarHeader,
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

export function AppSidebar({ meta, project, herdr, gitStatus }: SidebarProps) {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { setOpenMobile, state } = useSidebar()
  const collapsed = state === 'collapsed'

  const handleNav = () => {
    setOpenMobile(false)
  }

  const workspaceStatus = (name: string): string | null =>
    herdr?.workspaces.find((w) => w.label === name)?.agent_status ?? null

  // openAgent navigates to the Herdr tab of the agent's project (when its
  // workspace label is a known project) or the current/default project, with
  // the agent's operation panel pre-opened.
  const openAgent = (paneId: string, fallbackProject?: string) => {
    handleNav()
    const agentProject =
      fallbackProject && meta?.projects?.includes(fallbackProject) ? fallbackProject : null
    const target =
      agentProject || project || meta?.default_project || meta?.projects?.[0] || ''
    if (!target) return
    navigate(`${projectUrlFor(target, '/herdr')}?agent=${encodeURIComponent(paneId)}`)
  }

  return (
    <Sidebar collapsible="icon" className="border-r">
      <SidebarHeader className="gap-1">
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
            const wsLabel = herdr?.workspaces.find((w) => w.workspace_id === a.workspace_id)?.label
            const dir = dirName(a.cwd)
            const agentName = a.custom_name || (a.name && a.name !== 'agy' ? a.name : null)
            const tooltip = [
              a.custom_name ?? a.name,
              a.status,
              wsLabel,
              a.cwd ?? dir,
              a.title ?? '',
            ]
              .filter(Boolean)
              .join(' · ')
            return (
              <SidebarMenuItem key={a.pane_id}>
                <SidebarMenuButton
                  onClick={() => openAgent(a.pane_id, wsLabel)}
                  tooltip={tooltip}
                  className="sidebar-agent-row cursor-pointer"
                  data-testid={`sidebar-agent-${a.pane_id}`}
                >
                  <Bot />
                  <span className="flex min-w-0 flex-1 flex-col items-start gap-0.5">
                    <span className="flex w-full items-center gap-1.5">
                      {agentName ? (
                        <>
                          <span className="truncate text-[13px] font-medium">{agentName}</span>
                          {wsLabel && (
                            <span
                              className="truncate text-[11px] text-sidebar-foreground/60"
                              title={`workspace: ${wsLabel}`}
                            >
                              · {wsLabel}
                            </span>
                          )}
                        </>
                      ) : (
                        <>
                          {wsLabel && (
                            <span className="truncate text-[13px] font-medium">{wsLabel}</span>
                          )}
                          {dir && (
                            <span
                              className="truncate text-[11px] text-sidebar-foreground/60"
                              title={a.cwd ?? dir}
                            >
                              · {dir}
                            </span>
                          )}
                        </>
                      )}
                      <span
                        className={`ml-auto inline-block size-2 shrink-0 rounded-full ${statusDotClass(a.status)} ${a.status === 'working' ? 'animate-pulse' : ''}`}
                        role="img"
                        aria-label={`agent status ${a.status}`}
                      />
                    </span>
                    {!collapsed && a.title && (
                      <span
                        className="w-full text-[11px] leading-snug whitespace-normal break-words text-muted-foreground line-clamp-2"
                        title={a.title}
                      >
                        {a.title}
                      </span>
                    )}
                  </span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
          {herdr && herdr.available && herdr.agents.length === 0 && !collapsed ? (
            <p className="px-2 text-xs text-sidebar-foreground/60">No agents running.</p>
          ) : null}
        </SidebarMenu>
      </SidebarHeader>

      <SidebarRail />
    </Sidebar>
  )
}
