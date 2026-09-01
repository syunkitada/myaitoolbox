import { test, expect, type Locator } from '@playwright/test'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

const treeButton = (explorer: Locator, name: string) =>
  explorer.locator('.knowledge-tree').getByRole('button', { name, exact: true })

async function fillMonacoEditor(
  page: import('@playwright/test').Page,
  content: string,
  label = 'File editor',
) {
  // Monaco's input textarea is sized to 0xN (kept hidden to assistive tech),
  // so click the visible editor container that hosts the labeled input.
  const editor = page.locator('.monaco-editor', { has: page.getByLabel(label) })
  await editor.click()
  await page.keyboard.press('ControlOrMeta+A')
  await page.keyboard.insertText(content)
}

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
  await page.getByRole('button', { name: 'File actions' }).click()
  await expect(page.getByRole('menuitem', { name: 'Move' })).toBeVisible()
  await expect(page.getByRole('menuitem', { name: 'Duplicate' })).toBeVisible()
  await expect(page.getByRole('menuitem', { name: 'Delete' })).toBeVisible()
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
  await expect(outline.getByText('Graph')).toHaveCount(0)
})

test('the details pane stays visible while the file content scrolls', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  const outlineHeader = page.locator('.outline-header')
  await expect(outlineHeader).toBeVisible()
  const scroller = page.locator('.knowledge-files')
  const before = await outlineHeader.boundingBox()
  expect(before).not.toBeNull()

  await scroller.evaluate((el) => el.scrollBy(0, 600))

  // Confirm the file view really scrolled, so the assertion below is meaningful.
  const scrolled = await scroller.evaluate((el) => el.scrollTop)
  expect(scrolled).toBeGreaterThan(0)

  const after = await outlineHeader.boundingBox()
  expect(after).not.toBeNull()
  expect(after!.y).toBeCloseTo(before!.y, 0)
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
  await expect(page.getByRole('button', { name: 'Add adhoc' })).toBeVisible()
  await page.getByRole('button', { name: 'Open terminal' }).click()
  await expect(page.locator('.terminal-panel')).toBeVisible()
})

test('terminal button toggles show/hide without creating new terminals', async ({ page }) => {
  const btn = page.getByRole('button', { name: 'Open terminal' })
  const panel = page.locator('.terminal-panel')
  await btn.click()
  await expect(panel).toBeVisible()

  // Hide: the entire panel disappears from the screen.
  await btn.click()
  await expect(panel).toBeHidden()

  // Show again; still one session, not another new terminal.
  await btn.click()
  await expect(panel).toBeVisible()

  const tabs = await page.evaluate(() => {
    const m = JSON.parse(window.localStorage.getItem('mybox_terminals_v1') || '{}')
    const first = Object.values(m)[0] as { tabs?: unknown[] } | undefined
    return first?.tabs?.length ?? 0
  })
  expect(tabs).toBe(1)
})

test('the tab bar hide button hides the terminal panel', async ({ page }) => {
  await page.getByRole('button', { name: 'Open terminal' }).click()
  const panel = page.locator('.terminal-panel')
  await expect(panel).toBeVisible()

  await page.getByRole('button', { name: 'Hide terminal' }).click()
  await expect(panel).toBeHidden()
})

test('the tab bar maximize button makes the terminal fill the screen', async ({ page }) => {
  await page.getByRole('button', { name: 'Open terminal' }).click()
  const panel = page.locator('.terminal-panel')
  await expect(panel).toBeVisible()

  const rectBefore = await panel.boundingBox()
  expect(rectBefore).not.toBeNull()

  await page.getByRole('button', { name: 'Maximize terminal' }).click()
  const maximized = await panel.boundingBox()
  expect(maximized).not.toBeNull()
  const vw = await page.evaluate(() => ({ w: window.innerWidth, h: window.innerHeight }))
  expect(maximized!.x).toBeLessThanOrEqual(2)
  expect(maximized!.y).toBeLessThanOrEqual(2)
  expect(maximized!.width).toBeGreaterThanOrEqual(vw.w - 4)
  expect(maximized!.height).toBeGreaterThanOrEqual(vw.h - 4)

  await page.getByRole('button', { name: 'Restore terminal' }).click()
  await expect(panel).toBeVisible()
  const restored = await panel.boundingBox()
  expect(restored).not.toBeNull()
  expect(restored!.height).toBeLessThan(vw.h - 4)
})

test('dragging the resize handle grows the internal terminal', async ({ page }) => {
  await page.getByRole('button', { name: 'Open terminal' }).click()
  const panel = page.locator('.terminal-panel')
  await expect(panel).toBeVisible()

  const xterm = page.locator('.terminal-xterm')
  await expect(xterm).toBeVisible()
  const heightBefore = await xterm.evaluate((el) => el.clientHeight)
  expect(heightBefore).toBeGreaterThan(100)

  const handle = page.locator('[data-testid="terminal-resize-handle"]')
  const box = await handle.boundingBox()
  expect(box).not.toBeNull()
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2)
  await page.mouse.down()
  await page.mouse.move(box!.x + 20, box!.y - 140, { steps: 8 })
  await page.mouse.up()

  await expect
    .poll(async () => page.locator('.terminal-xterm').evaluate((el) => el.clientHeight))
    .toBeGreaterThan(heightBefore + 100)
})

test('terminal session persists across a browser reload', async ({ page }) => {
  await page.getByRole('button', { name: 'Open terminal' }).click()
  const panel = page.locator('.terminal-panel')
  await expect(panel).toBeVisible()

  // Focus the terminal and run a command whose echoed output can be verified
  // as replayed history after reconnecting to the same session on reload.
  await page.locator('.terminal-xterm').click()
  await page.keyboard.type('echo PERSIST_RELOAD_99')
  await page.keyboard.press('Enter')
  await expect(page.locator('.terminal-xterm')).toContainText('PERSIST_RELOAD_99')

  const sessionIdBefore = await page.evaluate(() => {
    const m = JSON.parse(window.localStorage.getItem('mybox_terminals_v1') || '{}')
    const first = Object.values(m)[0] as { tabs?: { sessionId?: string }[] } | undefined
    return first?.tabs?.[0]?.sessionId ?? null
  })
  expect(sessionIdBefore).toBeTruthy()

  await page.reload()
  await expect(page.locator('.terminal-panel')).toBeVisible()

  const sessionIdAfter = await page.evaluate(() => {
    const m = JSON.parse(window.localStorage.getItem('mybox_terminals_v1') || '{}')
    const first = Object.values(m)[0] as { tabs?: { sessionId?: string }[] } | undefined
    return first?.tabs?.[0]?.sessionId ?? null
  })
  expect(sessionIdAfter).toBe(sessionIdBefore)

  // Reattached to the same server-side session: historical output is replayed.
  await expect(page.locator('.terminal-xterm')).toContainText('PERSIST_RELOAD_99')
})

test('dashboard edits and saves a file', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await page.getByRole('button', { name: 'Edit' }).click()
  await fillMonacoEditor(page, '# Task tracking\n\nEdited content.\n')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Saved.')).toBeVisible()
  await expect(page.getByText('Edited content.')).toBeVisible()
})

test('dashboard edits frontmatter metadata separately from the body', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await page.getByRole('button', { name: 'Edit' }).click()
  // the metadata form is collapsed by default; expand it first
  await page.getByRole('button', { name: /Metadata/ }).click()
  await page.getByLabel('Metadata status').selectOption('done')
  await page.getByLabel('Metadata priority').selectOption('high')
  await page.getByLabel('Metadata tags').fill('docs, meta')
  await fillMonacoEditor(page, '# Tasks\n\nMetadata added.\n')
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
  await page.getByRole('button', { name: 'File actions' }).click()
  await page.getByRole('menuitem', { name: 'Move' }).click({ force: true })
  await expect(page.getByText('knowledge/tasks2.md', { exact: true })).toBeVisible()
  await expect(explorer).toContainText('tasks2.md')
})

test('dashboard duplicates a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept('knowledge/tasks2-copy.md'))
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks2.md').click()
  await page.getByRole('button', { name: 'File actions' }).click()
  await page.getByRole('menuitem', { name: 'Duplicate' }).click({ force: true })
  // Scope to the tree: the opened viewer's meta line shows the same path.
  await expect(treeButton(explorer, 'tasks2-copy.md')).toBeVisible()
})

test('dashboard deletes a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept())
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks2-copy.md').click()
  await page.getByRole('button', { name: 'File actions' }).click()
  await page.getByRole('menuitem', { name: 'Delete' }).click({ force: true })
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
  await tabs.getByRole('link', { name: 'Git' }).click()
  await expect(page).toHaveURL(/\/projects\/proj\/git$/)
  await expect(page.getByRole('heading', { name: 'No git repository' })).toBeVisible()
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
  await page.goto('/projects/proj/search')
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

test('favicon mirrors the aggregated agent status and updates dynamically', async ({ page }) => {
  await page.goto('/projects/proj/herdr')

  // The stub agent reports "working"; the favicon becomes its blue dot PNG.
  const favicon = page.locator('link[rel="icon"]')
  await expect(favicon).toHaveAttribute('href', /data:image\/png/)
  const workingHref = await favicon.getAttribute('href')
  expect(workingHref).toBeTruthy()

  // Once polled data flips the agent to "blocked" the favicon is redrawn.
  await page.route('**/api/herdr/overview', async (route) => {
    const res = await route.fetch()
    const body = { ...(await res.json()) }
    body.agents = [{ ...body.agents[0], status: 'blocked' }]
    await route.fulfill({ response: res, json: body })
  })
  await expect(favicon).not.toHaveAttribute('href', workingHref!, { timeout: 10000 })
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

  // the focused agent polls its terminal output every second
  const pre = detail.locator('pre')
  const before = await pre.innerText()
  await expect(pre).not.toHaveText(before, { timeout: 5000 })

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

test('clicking a sidebar agent of another project switches to that project first', async ({
  page,
}) => {
  const row = page.getByTestId('sidebar-agent-w8:p1')
  await expect(row).toContainText('other')
  await row.click()
  await expect(page).toHaveURL(/\/projects\/other\/herdr\?agent=w8%3Ap1$/)
  const detail = page.locator('.herdr-agent-detail')
  await expect(detail).toBeVisible()
  await expect(detail.locator('pre')).toContainText('stub output for w8:p1')
})

test('herdr offers a new tab when no tabs or panes exist at all', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()

  // close every visible tab; the last tab of a workspace also removes the
  // workspace. (Workspaces of other projects are not shown on this page and
  // do not count as tabs/panes of this project.)
  for (const tabId of ['w7:t1', 'w7:t2']) {
    const tab = page.getByTestId(`herdr-tab-${tabId}`)
    page.once('dialog', (d) => d.accept())
    await tab.hover()
    await tab.getByRole('button', { name: `Close tab ${tabId}` }).click()
    await expect(tab).toHaveCount(0)
  }

  // with nothing left, the empty state still offers tab creation
  const empty = page.getByTestId('herdr-no-workspace')
  await expect(empty).toContainText(`No herdr workspace found for project "proj"`)
  await expect(page.getByTestId('herdr-create-first-tab')).toBeVisible()

  page.once('dialog', (d) => d.accept('fresh'))
  await page.getByTestId('herdr-create-first-tab').click()

  // bootstrapping creates the project's first workspace with the named tab
  const workspace = page.locator('[data-testid^="herdr-workspace-"]')
  await expect(workspace).toHaveCount(1)
  await expect(workspace).toContainText('proj')
  await expect(page.locator('[data-testid^="herdr-tab-"]').filter({ hasText: 'fresh' })).toBeVisible()
})

test('graph renders knowledge nodes', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
})

test('board drag-and-drop changes task status and front matter', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()

  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Add adhoc' })).toBeVisible()

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

test('adhoc task appears on the board with an adhoc badge', async ({ page }) => {
  const created = await page.request.post('/api/tasks', {
    headers: { 'X-Project': 'proj' },
    data: { name: 'Review PR #99', type: 'adhoc', priority: 'urgent' },
  })
  expect(created.ok()).toBeTruthy()
  const adhoc = (await created.json()) as { id: string; type: string }
  expect(adhoc.type).toBe('adhoc')

  await page.goto('/projects/proj/dashboard')
  await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.getByText('Review PR #99')).toBeVisible()
  const card = page.locator('.board-card').filter({ hasText: 'Review PR #99' })
  await expect(card).toHaveClass(/adhoc-card/)
  await expect(card.locator('.badge', { hasText: 'adhoc' })).toBeVisible()

  const res = await page.request.get('/api/tasks?type=adhoc')
  const tasks = (await res.json()) as Array<{ id: string; type: string }>
  expect(tasks.some((t) => t.id === adhoc.id && t.type === 'adhoc')).toBeTruthy()
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

  test('terminal tab bar can hide the terminal on mobile', async ({ page }) => {
    await page.getByRole('button', { name: 'Open terminal' }).click()
    const panel = page.locator('.terminal-panel')
    await expect(panel).toBeVisible()

    await page.getByRole('button', { name: 'Hide terminal' }).click()
    await expect(panel).toBeHidden()
  })
})

test.describe('git tab', () => {
  test('initializes a repository, lists files, and commits the workspace', async ({ page }) => {
    await page.goto('/projects/proj/git')
    await expect(page.getByRole('heading', { name: 'No git repository' })).toBeVisible()
    await page.getByRole('button', { name: 'Initialize repository' }).click()
    await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()

    // Seed files are now visible as untracked (basenames; README.md appears
    // at the root and under knowledge/).
    await expect(page.locator('.knowledge-tree').getByRole('button', { name: 'README.md' })).toHaveCount(2)
    await expect(page.locator('.knowledge-tree').getByRole('button', { name: 'index.md' })).toHaveCount(1)

    // The commit box is disabled until a message is entered; committing with
    // "Stage all before commit" enabled captures the whole workspace.
    const commit = page.getByRole('button', { name: 'Commit', exact: true })
    await expect(commit).toBeDisabled()
    await page.getByTestId('git-commit-message').fill('Initial commit')
    await commit.click()
    await expect(page.getByTestId('git-output')).toContainText('Initial commit')
    await expect(page.getByText('Working tree is clean.')).toBeVisible()
  })

  test('amend rewrites the most recent commit message', async ({ page }) => {
    await page.goto('/projects/proj/git')
    const noRepo = page.getByRole('heading', { name: 'No git repository' })
    try {
      // Tolerate running this test on its own: seed a repository and a first
      // commit when the suite has not done so already.
      await noRepo.waitFor({ state: 'visible', timeout: 2000 })
      await page.getByRole('button', { name: 'Initialize repository' }).click()
      await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()
      await page.getByTestId('git-commit-message').fill('initial')
      await page.getByRole('button', { name: 'Commit', exact: true }).click()
      await expect(page.getByText('Working tree is clean.')).toBeVisible()
    } catch {
      // A repository (and commit) already exists.
    }

    page.once('dialog', (d) => d.accept())
    await page.getByTestId('git-commit-message').fill('amended subject')
    await page.getByRole('button', { name: 'Amend' }).click()
    await expect(page.getByTestId('git-output')).toContainText('amended subject')
    await expect(page.getByText('Working tree is clean.')).toBeVisible()
  })

  test('reports pull and push failures when there is no remote', async ({ page }) => {
    await page.goto('/projects/proj/git')
    // Initialize if this test ran without the initialization test; each test
    // must be runnable on its own, so tolerate either state.
    const noRepo = page.getByRole('heading', { name: 'No git repository' })
    try {
      await noRepo.waitFor({ state: 'visible', timeout: 2000 })
      await page.getByRole('button', { name: 'Initialize repository' }).click()
      await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()
    } catch {
      // The repository already exists from the previous test.
    }
    await page.getByRole('button', { name: 'Pull' }).click()
    await expect(page.getByTestId('git-output')).toContainText('no tracking information')
    await page.getByRole('button', { name: 'Push' }).click()
    await expect(page.getByTestId('git-output')).toContainText('No configured push destination')
  })
})

test.describe('git tab on a mobile viewport', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('working tree opens as a slide-over and stays reachable after selecting a file', async ({ page }) => {
    await page.request.post('/api/files', { data: { path: 'tmp-untracked.md' } })
    try {
      await page.goto('/projects/proj/git')
      // Initialize if this tested in isolation; otherwise a repo already exists.
      const noRepo = page.getByRole('heading', { name: 'No git repository' })
      try {
        await noRepo.waitFor({ state: 'visible', timeout: 2000 })
        await page.getByRole('button', { name: 'Initialize repository' }).click()
        await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()
      } catch {
        // The repository already exists from the git tab suite.
      }

      // On mobile the working tree is a slide-over, hidden by default.
      await expect(page.locator('.explorer')).toHaveCount(0)
      await page.getByRole('button', { name: 'Toggle file explorer' }).click()
      await expect(page.locator('.explorer')).toBeVisible()

      // Picking a file closes the slide-over and fills the pane with its diff.
      await treeButton(page.locator('.explorer'), 'tmp-untracked.md').click()
      await expect(page.getByTestId('git-selected-file')).toHaveText('tmp-untracked.md')
      await expect(page.locator('.explorer')).toHaveCount(0)

      // The toggle in the diff toolbar brings the working tree back.
      await page.getByRole('button', { name: 'Toggle file explorer' }).click()
      await expect(page.locator('.explorer')).toBeVisible()
      await expect(treeButton(page.locator('.explorer'), 'tmp-untracked.md')).toHaveClass(/active/)
    } finally {
      await page.request.post('/api/files/delete', { data: { path: 'tmp-untracked.md' } })
    }
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
