import { test, expect, type Locator } from '@playwright/test'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

const treeButton = (explorer: Locator, name: string) =>
  explorer.locator('.knowledge-tree').getByRole('button', { name, exact: true })

test.beforeEach(async ({ page }) => {
  await page.goto('/projects/proj/dashboard')
})

test('dashboard shows project file explorer with README by default', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  await expect(
    page.getByText('This project tracks tasks and knowledge.', { exact: true }),
  ).toBeVisible()
  const explorer = page.locator('.knowledge-explorer')
  await expect(explorer).toContainText('README.md')
  await expect(explorer).toContainText('knowledge')
  await expect(explorer).toContainText('tasks')
})

test('dashboard shows a task status badge on the containing directory', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand tasks' }).click()
  const dirRow = explorer.locator('.knowledge-tree-row', { hasText: 'e2e-status-change-target' })
  await expect(dirRow.locator('.badge.status-doing')).toBeVisible()
  await expect(dirRow.locator('.badge.status-doing')).toHaveText('doing')
  await explorer.getByRole('button', { name: 'Expand e2e-status-change-target' }).click()
  const taskRow = explorer.locator('.knowledge-tree-row', { hasText: 'task.md' })
  await expect(taskRow.locator('.badge')).toHaveCount(0)
})

test('dashboard opens a markdown file from the explorer', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await expect(explorer).toContainText('tasks.md')
  await treeButton(explorer, 'tasks.md').click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  await expect(
    treeButton(explorer, 'tasks.md'),
  ).toHaveClass(/active/)
})

test('dashboard opens a directory README from the explorer', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'knowledge').click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge$/)
  await expect(page.getByText('All knowledge lives here.')).toBeVisible()
  await expect(page.locator('.markdown-body a', { hasText: 'docs' })).toHaveAttribute(
    'href',
    '/projects/proj/dashboard/files/knowledge/docs',
  )
  await expect(treeButton(explorer, 'knowledge')).toHaveClass(/active/)
  await expect(page.getByRole('button', { name: 'Edit' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Rename' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Move' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Duplicate' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Delete' })).toBeVisible()
})

test('dashboard lists a directory that has no README', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'docs').click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/docs$/)
  await expect(page.getByRole('heading', { name: 'Directories', level: 2 })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Files', level: 2 })).toBeVisible()
  await expect(page.locator('.markdown-body a', { hasText: 'recipes/' })).toHaveAttribute(
    'href',
    '/projects/proj/dashboard/files/knowledge/docs/recipes',
  )
  await expect(page.locator('.markdown-body a', { hasText: 'guide.md' })).toHaveAttribute(
    'href',
    '/projects/proj/dashboard/files/knowledge/docs/guide.md',
  )
})

test('a directory link in a README opens the subdirectory listing', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'knowledge').click()
  await expect(page.getByText('All knowledge lives here.')).toBeVisible()
  await page.locator('.markdown-body a', { hasText: 'docs' }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/docs$/)
  await expect(page.getByRole('heading', { name: 'Files', level: 2 })).toBeVisible()
  await expect(page.locator('.markdown-body a', { hasText: 'guide.md' })).toBeVisible()
})

test('dashboard keeps the open file in the URL across a reload', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/tasks\.md$/)
  await page.reload()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  await expect(
    treeButton(explorer, 'tasks.md'),
  ).toHaveClass(/active/)
})

test('dashboard shows a content outline for markdown files', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  const outline = page.locator('.outline')
  await expect(outline).toContainText('Content')
  await expect(outline.getByRole('link', { name: 'Tasks' })).toBeVisible()
})

test('dashboard toggles the details sidebar', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  const pane = page.locator('.outline-pane')
  await expect(pane).toHaveAttribute('data-outline-open', 'true')
  await page.getByRole('button', { name: 'Toggle details' }).click()
  await expect(pane).toHaveAttribute('data-outline-open', 'false')
  await page.getByRole('button', { name: 'Toggle details' }).click()
  await expect(pane).toHaveAttribute('data-outline-open', 'true')
  await expect(page.locator('.outline')).toContainText('Content')
})

test('dashboard toggles the file explorer', async ({ page }) => {
  const pane = page.locator('.explorer-pane')
  await expect(pane).toHaveAttribute('data-explorer-open', 'true')
  await page.getByRole('button', { name: 'Toggle file explorer' }).click()
  await expect(pane).toHaveAttribute('data-explorer-open', 'false')
  await page.getByRole('button', { name: 'Toggle file explorer' }).click()
  await expect(pane).toHaveAttribute('data-explorer-open', 'true')
})

test('nav bar file actions open a terminal', async ({ page }) => {
  await expect(page.getByRole('button', { name: 'New file' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()
  await page.getByRole('button', { name: 'Open terminal' }).click()
  await expect(page.locator('.terminal-panel')).toBeVisible()
})

test('dashboard edits and saves a file', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await page.getByRole('button', { name: 'Edit' }).click()
  const editor = page.getByLabel('File editor')
  await editor.fill('# Task tracking\n\nEdited content.\n')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Saved.')).toBeVisible()
  await expect(page.getByText('Edited content.')).toBeVisible()
})

test('dashboard edits frontmatter metadata separately from the body', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await page.getByRole('button', { name: 'Edit' }).click()
  await page.getByLabel('Metadata status').selectOption('done')
  await page.getByLabel('Metadata priority').selectOption('high')
  await page.getByLabel('Metadata tags').fill('docs, meta')
  await page.getByLabel('File editor').fill('# Tasks\n\nMetadata added.\n')
  await page.getByRole('button', { name: 'Save' }).click()

  await expect(page.locator('.frontmatter-card .badge.status-done')).toBeVisible()
  await expect(page.locator('.frontmatter-card .badge.priority-high')).toBeVisible()
  await expect(page.locator('.frontmatter-card .badge.tag').filter({ hasText: 'docs' })).toBeVisible()
  await expect(page.getByText('Metadata added.')).toBeVisible()

  const res = await page.request.get('/api/files/content?path=tasks.md')
  const body = (await res.json()) as { content: string }
  expect(body.content).toContain('status: done')
  expect(body.content).toContain('priority: high')
  expect(body.content).toContain('tags')
  expect(body.content).toContain('docs')
  expect(body.content).toContain('Metadata added.')

  const row = explorer.locator('.knowledge-tree-row', { hasText: 'tasks.md' })
  await expect(row.locator('.badge.status-done')).toBeVisible()
})

test('dashboard favorites a file from the file pane', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await page.getByRole('button', { name: '☆ Favorite' }).click()
  await expect(page.getByRole('button', { name: '★ Favorite' })).toBeVisible()
  const favorites = explorer.locator('.explorer-section').filter({ hasText: 'Favorites' })
  await expect(favorites).toContainText('tasks.md')
})

test('dashboard records recently opened files', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  const recent = explorer.locator('.explorer-section').filter({ hasText: 'Recent' })
  await expect(recent).toContainText('tasks.md')
})

test('dashboard moves a file by dragging onto a directory', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await expect(explorer).toContainText('tasks.md')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks.md')
    .dragTo(explorer.getByRole('button', { name: 'Collapse knowledge' }))
  await explorer.locator('.search-bar input').fill('knowledge/tasks')
  const list = explorer.locator('.file-list')
  await expect(list.getByRole('button', { name: 'knowledge/tasks.md' })).toBeVisible()
})

test('dashboard renames a file via Move', async ({ page }) => {
  page.on('dialog', (d) => d.accept('knowledge/tasks2.md'))
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks.md').click()
  await page.getByRole('button', { name: 'Move' }).click()
  await expect(page.getByText('knowledge/tasks2.md', { exact: true })).toBeVisible()
  await expect(explorer).toContainText('tasks2.md')
})

test('dashboard duplicates a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept('knowledge/tasks2-copy.md'))
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks2.md').click()
  await page.getByRole('button', { name: 'Duplicate' }).click()
  // Scope to the tree: the opened viewer's meta line shows the same path.
  await expect(treeButton(explorer, 'tasks2-copy.md')).toBeVisible()
})

test('dashboard deletes a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept())
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks2-copy.md').click()
  await page.getByRole('button', { name: 'Delete' }).click()
  await expect(
    page.getByText('Select a file from the explorer to view it here.', { exact: true }),
  ).toBeVisible()
  await expect(explorer).not.toContainText('tasks2-copy.md')
})

test('clicking the mybox brand returns to the unselected projects page', async ({ page }) => {
  await page.locator('.sidebar-nav').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  await page.getByRole('button', { name: 'Go to top' }).click()
  await expect(page.getByRole('heading', { name: 'Workspaces', level: 1 })).toBeVisible()
  await expect(page.locator('.sidebar-projects')).toContainText('proj')
  await expect(page.locator('.sidebar-nav').getByRole('link', { name: 'Dashboard' })).toHaveCount(0)
})

test('project tabs switch between files, board and graph', async ({ page }) => {
  const tabs = page.locator('.project-tabs')
  await expect(tabs.getByRole('link', { name: 'Files' })).toBeVisible()
  await tabs.getByRole('link', { name: 'Board' }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/board$/)
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  await expect(tabs.getByRole('link', { name: 'Board' })).toHaveClass(/bg-accent/)
  await tabs.getByRole('link', { name: 'Graph' }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/graph$/)
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
  await tabs.getByRole('link', { name: 'Files' }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard/)
  await expect(tabs.getByRole('link', { name: 'Files' })).toHaveClass(/bg-accent/)
})

test('sidebar lists projects and switches between them', async ({ page }) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-side-'))
  const res = await page.request.post('/api/projects', { data: { path: dir } })
  expect(res.ok()).toBeTruthy()
  await page.reload()
  const list = page.locator('.sidebar-projects')
  await expect(list).toContainText('proj')
  const other = path.basename(dir)
  await list.getByRole('button', { name: other }).click()
  await expect(page).toHaveURL(new RegExp(`/projects/${other}/dashboard$`))
})

test('search finds knowledge and opens it in the dashboard', async ({ page }) => {
  await page.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/search$/)
  await page.getByPlaceholder('Search…').fill('phase6')
  await page.getByPlaceholder('Search…').press('Enter')
  await expect(page.getByRole('heading', { name: 'Search' })).toBeVisible()
  await page.getByRole('button', { name: 'Phase 6' }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/notes\/phase6\.md$/)
  await expect(page.getByText('The HTTP API is done.')).toBeVisible()
})

test('sidebar board shows tasks across projects', async ({ page }) => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-cross-'))
  const added = await page.request.post('/api/projects', { data: { path: dir } })
  expect(added.ok()).toBeTruthy()
  const other = path.basename(dir)
  const taskName = `Cross task ${other}`
  const created = await page.request.post('/api/tasks', {
    headers: { 'X-Project': other },
    data: { name: taskName },
  })
  expect(created.ok()).toBeTruthy()

  await page.goto('/projects/proj/dashboard')
  await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.locator('.board-card').first()).toBeVisible()
  await page.locator('.sidebar-nav').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page).toHaveURL(/\/board$/)
  await expect(page.getByText('全プロジェクト横断ビュー')).toBeVisible()
  const cards = page.locator('.board-card')
  await expect(cards.filter({ hasText: taskName })).toBeVisible()
  await expect(cards.filter({ hasText: 'Ship the web UI' })).toBeVisible()
})

test('herdr tab shows workspaces and operates agents', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  await expect(page.getByRole('heading', { name: 'Herdr', level: 1 })).toBeVisible()
  await expect(page.getByTestId('herdr-workspace-w7')).toContainText('proj')
  await expect(page.getByTestId('herdr-workspace-w7')).toContainText('working')

  await page.locator('.herdr-agent-row').first().click()
  const detail = page.locator('.herdr-agent-detail')
  await expect(detail).toBeVisible()
  await expect(detail.locator('pre')).toContainText('stub output for w7:p1')

  await page.getByTestId('herdr-prompt-input').fill('run the tests please')
  await detail.getByRole('button', { name: 'Send' }).click()
  await expect(detail.locator('.herdr-prompt-notice')).toContainText('prompt submitted')
  await expect(detail.locator('pre')).toContainText('last prompt: run the tests please')
})

test('herdr tab and pane operations work end to end', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  const ws = page.getByTestId('herdr-workspace-w7')
  await expect(ws).toBeVisible()
  await expect(page.getByTestId('herdr-tab-w7:t1')).toBeVisible()
  await expect(page.getByTestId('herdr-tab-w7:t2')).toContainText('2:')
  // the first tab is selected by default; focus is webui-managed
  await expect(page.getByTestId('herdr-tab-w7:t1')).toHaveAttribute('data-active', 'true')

  // create a tab via the prompt dialog
  page.once('dialog', (d) => d.accept('build'))
  await ws.getByRole('button', { name: '+ New Tab' }).click()
  const newTab = page.getByTestId('herdr-tab-w7:t3')
  await expect(newTab).toContainText('build')

  // selecting a tab stores the focus in the URL instead of calling herdr
  await newTab.click()
  await expect(newTab).toHaveAttribute('data-active', 'true')
  await expect(page.getByTestId('herdr-tab-w7:t1')).toHaveAttribute('data-active', 'false')
  await expect(page).toHaveURL(/tab=w7(:|%3A)t3/)

  // clicking a pane header focuses it (also persisted in the URL)
  await expect(page.getByTestId('herdr-pane-w7:p3')).toBeVisible()
  await page.getByTestId('herdr-pane-header-w7:p3').click()
  await expect(page.getByTestId('herdr-pane-w7:p3')).toHaveAttribute('data-focused', 'true')
  await expect(page).toHaveURL(/pane=w7(:|%3A)p3/)

  // switching tabs clears the pane focus
  await page.getByTestId('herdr-tab-w7:t2').click()
  await expect(page.getByTestId('herdr-tab-w7:t2')).toHaveAttribute('data-active', 'true')
  const p2 = page.getByTestId('herdr-pane-w7:p2')
  await expect(p2).toBeVisible()
  await expect(page).not.toHaveURL(/pane=/)

  // reloading restores exactly the same tab/pane focus from the URL
  await page.getByTestId('herdr-pane-header-w7:p2').click()
  await expect(p2).toHaveAttribute('data-focused', 'true')
  await page.reload()
  await expect(page.getByTestId('herdr-tab-w7:t2')).toHaveAttribute('data-active', 'true')
  await expect(page.getByTestId('herdr-pane-w7:p2')).toHaveAttribute('data-focused', 'true')

  // the focused pane polls its terminal output every second
  const p2Pre = p2.locator('pre')
  const before = await p2Pre.innerText()
  await expect(p2Pre).not.toHaveText(before, { timeout: 5000 })

  // while auto reloading, the terminal stays scrolled to its latest output
  await expect
    .poll(async () => p2Pre.evaluate((el) => el.scrollHeight - el.clientHeight - el.scrollTop), {
      timeout: 5000,
    })
    .toBeLessThan(32)

  // auto reload can be toggled off; the output then stops refreshing
  const toggle = page.getByTestId('herdr-auto-reload-toggle')
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'false')
  // let any in-flight reload land before freezing expectations
  await page.waitForTimeout(1500)
  const frozen = await p2Pre.innerText()
  await page.waitForTimeout(2500)
  await expect(p2Pre).toHaveText(frozen)
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')

  // split an existing pane downward; the new pane appears under the same tab
  await p2.getByRole('button', { name: 'Split w7:p2 down' }).click()
  const p4 = page.getByTestId('herdr-pane-w7:p4')
  await expect(p4).toBeVisible()

  // pane terminal output is loaded automatically while open
  await expect(page.locator('.herdr-pane-detail').locator('pre').first()).toContainText(
    'stub pane output for',
  )

  // rename the split pane
  page.once('dialog', (d) => d.accept('logs'))
  await p4.getByRole('button', { name: 'Rename', exact: true }).click()
  await expect(p4).toContainText('logs')

  // close the pane and then the tab (both confirm dialogs)
  page.once('dialog', (d) => d.accept())
  await p4.locator('.herdr-pane-close').click()
  await expect(p4).toHaveCount(0)

  page.once('dialog', (d) => d.accept())
  await newTab.hover()
  await newTab.getByRole('button', { name: `Close tab w7:t3` }).click()
  await expect(newTab).toHaveCount(0)
})

test('sidebar shows workspace status and herdr agents', async ({ page }) => {
  await page.goto('/projects/proj/dashboard')
  const projRow = page
    .locator('.sidebar-projects button')
    .filter({ hasText: 'proj' })
    .first()
  await expect(
    projRow.locator('.herdr-workspace-status[aria-label="workspace status working"]'),
  ).toBeVisible()

  const agents = page.locator('.sidebar-agents')
  await expect(agents).toContainText('opencode')
  await expect(agents.locator('[aria-label="agent status working"]').first()).toBeVisible()
  // agent row details: workspace label, cwd directory name and terminal title
  const row = page.getByTestId('sidebar-agent-w7:p1')
  await expect(row).toContainText('proj')
  await expect(row).toContainText('proj-dir')
  await expect(row).toContainText('OC | stub agent')
})

test('clicking a sidebar agent opens its operation panel in the herdr tab', async ({ page }) => {
  await page.goto('/projects/proj/dashboard')
  await page.getByTestId('sidebar-agent-w7:p1').click()
  await expect(page).toHaveURL(/\/projects\/proj\/herdr\?agent=w7%3Ap1$/)
  const detail = page.locator('.herdr-agent-detail')
  await expect(detail).toBeVisible()
  await expect(detail.locator('pre')).toContainText('stub output for w7:p1')

  await page.getByTestId('herdr-prompt-input').fill('hello from sidebar')
  await detail.getByRole('button', { name: 'Send' }).click()
  await expect(detail.locator('pre')).toContainText('last prompt: hello from sidebar')
})

test('graph renders knowledge nodes', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
})

test('board drag-and-drop changes task status and front matter', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()

  const card = page.getByText('e2e-status-change-target')
  await expect(card).toBeVisible()
  const doneColumn = page.getByTestId('column-done')

  const cardBox = await card.boundingBox()
  const doneBox = await doneColumn.boundingBox()
  if (!cardBox || !doneBox) throw new Error('missing boxes')

  await page.mouse.move(cardBox.x + cardBox.width / 2, cardBox.y + cardBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(doneBox.x + doneBox.width / 2, doneBox.y + doneBox.height / 2, { steps: 10 })
  await page.mouse.up()

  await expect(doneColumn.getByText('e2e-status-change-target')).toBeVisible()

  const res = await page.request.get('/api/tasks')
  const tasks = (await res.json()) as Array<{ id: string; status: string }>
  const moved = tasks.find((t) => t.id === 'e2e-status-change-target')
  expect(moved?.status).toBe('done')
})

test.describe('mobile viewport', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test.beforeEach(async ({ page }) => {
    await page.goto('/projects/proj/dashboard/files/tasks.md')
  })

  test('hides the details sidebar by default and opens it as a slide-over', async ({ page }) => {
    await expect(page.locator('.outline')).toHaveCount(0)
    await page.getByRole('button', { name: 'Toggle details' }).click()
    await expect(page.locator('.outline')).toContainText('Content')
    await page.getByRole('button', { name: 'Close' }).click()
    await expect(page.locator('.outline')).toHaveCount(0)
  })

  test('file explorer opens as a slide-over', async ({ page }) => {
    await page.goto('/projects/proj/dashboard/files/README.md')
    await expect(page.locator('.explorer')).toHaveCount(0)
    await page.getByRole('button', { name: 'Toggle file explorer' }).click()
    await expect(page.locator('.explorer')).toBeVisible()
    await treeButton(page.locator('.explorer'), 'README.md').click()
    await expect(page.locator('.explorer')).toHaveCount(0)
  })
})

test.describe('project selection at /', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
  })

  test('shows projects with an unselected project box and a Workspaces-only menu', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Workspaces' })).toBeVisible()
    const nav = page.locator('.sidebar-nav')
    await expect(nav.getByRole('link', { name: 'Workspaces' })).toBeVisible()
    await expect(nav.getByRole('link', { name: 'Board' })).toBeVisible()
    await expect(nav.getByRole('link', { name: 'Dashboard' })).toHaveCount(0)
    await expect(page.locator('.project-tabs')).toHaveCount(0)
    await expect(page.locator('.sidebar-projects')).toContainText('proj')
  })

  test('offers real path candidates when creating a project', async ({ page }) => {
    const input = page.getByLabel('Project path')
    await input.click()
    const list = page.locator('.path-candidates')
    await expect(list).toBeVisible()
    await expect(list.locator('li').first()).toBeVisible()
    await input.fill(os.homedir())
    await expect(list.locator('li').first()).toContainText(os.homedir())
  })

  test('creates and deletes a project from a real path', async ({ page }) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-'))
    await page.getByLabel('Project path').fill(dir)
    await page.getByRole('button', { name: 'Create project' }).click()
    const row = page.locator('.projects-table tbody tr', { hasText: path.basename(dir) })
    await expect(row).toBeVisible()
    await expect(row.locator('.project-path')).toHaveText(dir)

    page.once('dialog', (d) => d.accept())
    await row.getByRole('button', { name: 'Delete' }).click()
    await expect(
      page.locator('.projects-table tbody tr', { hasText: path.basename(dir) }),
    ).toBeHidden()
  })
})
