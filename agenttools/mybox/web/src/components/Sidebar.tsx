import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { Meta } from '../api/client'
import { clearProject, encodePath, projectUrl, setProject } from '../utils/routes'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { Box, Boxes, Clock, FolderHeart, LayoutDashboard, Network, Search, SquareKanban } from 'lucide-react'

interface SidebarProps {
  meta: Meta | null
  project: string
}

export function AppSidebar({ meta, project }: SidebarProps) {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { state, setOpenMobile } = useSidebar()

  const handleNav = () => {
    setOpenMobile(false)
  }

  const activePath = (to: string) =>
    pathname === to || pathname.startsWith(to.endsWith('/') ? to : to + '/')

  const openFile = (p: string) => {
    navigate(projectUrl(`/dashboard/files/${encodePath(p)}`))
    handleNav()
  }

  return (
    <Sidebar collapsible="icon" className="border-r">
      <SidebarHeader className="gap-1">
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              size="lg"
              onClick={clearProject}
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

        {meta && meta.projects.length > 0 && (
          <div className="sidebar-brand px-1 group-data-[collapsible=icon]:hidden">
            <select
              value={project}
              onChange={(e) => {
                if (e.target.value) setProject(e.target.value)
              }}
              aria-label="Switch project"
              className="h-8 w-full min-w-0 rounded-md border border-input bg-background px-2 text-sm text-foreground outline-none transition-[color,box-shadow] focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
            >
              {!project && <option value="">未選択</option>}
              {meta.projects.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>
        )}

        <SidebarMenu className="sidebar-nav mt-1">
          {project ? (
            <>
              {state === 'collapsed' && (
                <SidebarMenuItem>
                  <SidebarMenuButton asChild tooltip="Search">
                    <NavLink to={projectUrl('/search')} onClick={handleNav}>
                      <Search />
                      <span>Search</span>
                    </NavLink>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              )}
              <SidebarMenuItem>
                <SidebarMenuButton
                  asChild
                  tooltip="Dashboard"
                  isActive={activePath(projectUrl('/dashboard'))}
                >
                  <NavLink to={projectUrl('/dashboard')} onClick={handleNav}>
                    <LayoutDashboard />
                    <span>Dashboard</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild tooltip="Board" isActive={activePath(projectUrl('/board'))}>
                  <NavLink to={projectUrl('/board')} onClick={handleNav}>
                    <SquareKanban />
                    <span>Board</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild tooltip="Graph" isActive={activePath(projectUrl('/graph'))}>
                  <NavLink to={projectUrl('/graph')} onClick={handleNav}>
                    <Network />
                    <span>Graph</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </>
          ) : (
            <>
              <SidebarMenuItem>
                <SidebarMenuButton asChild tooltip="Projects" isActive={activePath('/projects')}>
                  <NavLink to="/projects" end onClick={handleNav}>
                    <Boxes />
                    <span>Projects</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
              <SidebarMenuItem>
                <SidebarMenuButton asChild tooltip="Board" isActive={activePath('/board')}>
                  <NavLink to="/board" end onClick={handleNav}>
                    <SquareKanban />
                    <span>Board</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
            </>
          )}
        </SidebarMenu>
      </SidebarHeader>

      {project && (
        <SidebarContent>
          <SidebarGroup className="sidebar-section group-data-[collapsible=icon]:hidden">
            <SidebarGroupLabel>Favorites</SidebarGroupLabel>
            <SidebarGroupContent>
              {(meta?.favorites ?? []).length === 0 ? (
                <p className="px-2 text-xs text-sidebar-foreground/60">No favorites yet.</p>
              ) : (
                <SidebarMenu className="sidebar-list">
                  {(meta?.favorites ?? []).map((p) => (
                    <SidebarMenuItem key={p}>
                      <SidebarMenuButton onClick={() => openFile(p)} tooltip={p} className="cursor-pointer">
                        <FolderHeart className="text-sidebar-accent-foreground/70" />
                        <span className="truncate">{p}</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              )}
            </SidebarGroupContent>
          </SidebarGroup>

          <SidebarGroup className="sidebar-section group-data-[collapsible=icon]:hidden">
            <SidebarGroupLabel>Recent</SidebarGroupLabel>
            <SidebarGroupContent>
              {(meta?.recent_files ?? []).length === 0 ? (
                <p className="px-2 text-xs text-sidebar-foreground/60">No recent files.</p>
              ) : (
                <SidebarMenu className="sidebar-list">
                  {(meta?.recent_files ?? []).map((p) => (
                    <SidebarMenuItem key={p}>
                      <SidebarMenuButton onClick={() => openFile(p)} tooltip={p} className="cursor-pointer">
                        <Clock className="text-sidebar-accent-foreground/70" />
                        <span className="truncate">{p}</span>
                      </SidebarMenuButton>
                    </SidebarMenuItem>
                  ))}
                </SidebarMenu>
              )}
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
      )}

      {project && (
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                onClick={clearProject}
                tooltip="Projects"
                className="cursor-pointer text-sidebar-foreground/80"
              >
                <Boxes />
                <span>Projects</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}

      <SidebarRail />
    </Sidebar>
  )
}
