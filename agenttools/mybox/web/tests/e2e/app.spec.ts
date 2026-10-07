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

// The web UI uses a custom modal (AppDialogs) instead of native browser
// prompt/confirm/alert dialogs. This helper accepts it: it fills a value when
// one is given (prompt) and otherwise just confirms (confirm/alert).
async function acceptAppDialog(
  page: import('@playwright/test').Page,
  value?: string,
) {
  const dialog = page.getByTestId('app-dialog')
  await expect(dialog).toBeVisible()
  if (value !== undefined) {
    await dialog.getByTestId('app-dialog-input').fill(value)
  }
  await dialog.getByTestId('app-dialog-ok').click()
}

async function openHerdrWorkspaces(page: import('@playwright/test').Page) {
  const toggle = page.getByTestId('herdr-workspaces-toggle')
  await expect(toggle).toBeVisible()
  await toggle.click()
}

async function closeAgentSidebar(page: import('@playwright/test').Page) {
  const sheet = page.getByTestId('agent-sidebar-sheet')
  if (await sheet.count()) {
    await page.keyboard.press('Escape')
    await expect(sheet).toHaveCount(0)
    return
  }
  const close = page.getByTestId('agent-sidebar-toggle')
  if (await close.count() && (await close.getAttribute('aria-label')) === 'Close agent sidebar') {
    await close.click()
  }
}

async function removeProjectGitRepo(page: import('@playwright/test').Page) {
  const projectsRes = await page.request.get('/api/projects')
  const projects = (await projectsRes.json()) as Array<{ name: string; path: string }>
  const project = projects.find((item) => item.name === 'proj')
  if (project) fs.rmSync(path.join(project.path, '.git'), { recursive: true, force: true })
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
  await expect(explorer).toContainText('_tasks')
})

test('dashboard shows a task status badge on the containing directory', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand _tasks' }).click()
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

test('dashboard opens a file in the reference pane from the explorer context menu', async ({ page }) => {
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  const reference = treeButton(explorer, 'tasks.md')
  await reference.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Open in reference pane' }).click()

  await expect(page.getByTestId('reference-file-viewer-pane')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Close reference pane' })).toBeVisible()
  await expect(page.locator('.file-viewer-split')).toBeVisible()

  const mainPane = page.getByTestId('main-file-viewer-pane')
  await mainPane.getByRole('button', { name: 'Toggle details' }).dispatchEvent('click')
  await mainPane.getByRole('button', { name: 'Swap main and reference panes' }).click()
  await expect.poll(() => page.url()).toContain('/files/tasks.md')
  await expect(page.getByTestId('main-file-viewer-pane')).toContainText('tasks.md')
  await expect(page.getByTestId('reference-file-viewer-pane')).toContainText('README.md')
})

test('dashboard keeps the reference pane when the main file becomes the same file', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  const reference = treeButton(explorer, 'tasks.md')
  await reference.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Open in reference pane' }).click()

  await reference.click()
  await expect(page.getByTestId('main-file-viewer-pane')).toContainText('Task tracking lives here.')
  await expect(page.getByTestId('reference-file-viewer-pane')).toContainText('Task tracking lives here.')
  await expect(page.getByRole('button', { name: 'Close reference pane' })).toBeVisible()
})

test('dashboard compares a changed file with its Git diff', async ({ page }) => {
  const filePath = 'git-diff-viewer-test.txt'
  const fileContent = Array.from({ length: 300 }, (_, index) => `current file line ${index + 1}`).join('\n') + '\n'
  await page.request.post('/api/files', { data: { path: filePath } })
  await page.request.put('/api/files/content', {
    data: { path: filePath, content: fileContent },
  })
  let initializedRepo = false

  try {
    const gitStatus = await page.request.get('/api/git/status')
    if (!(await gitStatus.json()).is_repo) {
      await page.request.post('/api/git/init')
      initializedRepo = true
    }
    await page.goto(`/projects/proj/dashboard/files/${filePath}`)
    const explorer = page.locator('.knowledge-explorer')
    await treeButton(explorer, filePath).click()

    const gitButton = page.getByRole('button', { name: 'Show Git diff' })
    await expect(gitButton).toBeVisible()
    await gitButton.click()

    await expect(page.getByRole('button', { name: 'Hide Git diff' })).toBeVisible()
    await expect(page.getByTestId('git-file-diff')).toContainText('Untracked')
    await expect(page.getByTestId('git-file-diff')).toContainText('+current file line 1')

    const contentPane = page.getByTestId('git-file-content-pane')
    const diffPane = page.getByTestId('git-file-diff-pane')
    await expect.poll(() => contentPane.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true)
    await expect.poll(() => diffPane.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true)

    const diffBefore = await diffPane.evaluate((el) => el.scrollTop)
    await contentPane.evaluate((el) => { el.scrollTop = el.scrollHeight })
    expect(await contentPane.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
    expect(await diffPane.evaluate((el) => el.scrollTop)).toBe(diffBefore)

    const contentBefore = await contentPane.evaluate((el) => el.scrollTop)
    await diffPane.evaluate((el) => { el.scrollTop = el.scrollHeight })
    expect(await diffPane.evaluate((el) => el.scrollTop)).toBeGreaterThan(0)
    expect(await contentPane.evaluate((el) => el.scrollTop)).toBe(contentBefore)
  } finally {
    await page.request.post('/api/files/delete', { data: { path: filePath } })
    if (initializedRepo) {
      const projectsRes = await page.request.get('/api/projects')
      const projects = (await projectsRes.json()) as Array<{ name: string; path: string }>
      const project = projects.find((item) => item.name === 'proj')
      if (project) fs.rmSync(path.join(project.path, '.git'), { recursive: true, force: true })
    }
  }
})

test('markdown copy button sits on the file name row and switches formats in its dialog', async ({ page }) => {
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()

  const fileNameRow = page.locator('.meta-line')
  const copyButton = fileNameRow.getByRole('button', { name: 'Copy file contents' })
  await expect(copyButton).toBeVisible()
  await expect(copyButton).toHaveText('')
  const fileNameBox = await fileNameRow.getByText('tasks.md', { exact: true }).boundingBox()
  const copyButtonBox = await copyButton.boundingBox()
  expect(fileNameBox).not.toBeNull()
  expect(copyButtonBox).not.toBeNull()
  expect(copyButtonBox!.x).toBeGreaterThan(fileNameBox!.x + fileNameBox!.width)
  await expect(page.getByRole('button', { name: 'Copy Text' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Copy Jira' })).toHaveCount(0)

  await copyButton.click()
  const dialog = page.getByRole('dialog', { name: 'Copy file contents' })
  await expect(dialog.getByRole('textbox', { name: 'Text copy preview' })).toHaveValue(/# Tasks/)
  await dialog.getByRole('tab', { name: 'Jira' }).click()
  await expect(dialog.getByRole('textbox', { name: 'Jira copy preview' })).toHaveValue(/h1\. Tasks/)
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
  await expect(page.locator('.knowledge-pane').getByRole('button', { name: 'Rename' })).toHaveCount(0)
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
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  const outline = page.locator('.outline')
  await expect(outline).toContainText('Contents')
  await expect(outline.getByRole('link', { name: 'Tasks' })).toBeVisible()
  await expect(outline.getByText('Graph')).toHaveCount(0)
})

test('the details pane stays visible while the file content scrolls', async ({ page }) => {
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  const outlineHeader = page.locator('.outline-header')
  await expect(outlineHeader).toBeVisible()
  await expect(outlineHeader).toHaveText('Outline')
  await expect(outlineHeader).not.toContainText('On this page')
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
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  const pane = page.locator('.outline-pane')
  const detailsToggle = page.getByRole('button', { name: 'Toggle details' })
  await expect(pane).toHaveAttribute('data-outline-open', 'true')
  await expect(detailsToggle).not.toHaveClass(/text-primary/)
  await expect(detailsToggle.locator('svg')).toHaveClass(/lucide-panel-right-close/)
  await detailsToggle.click()
  await expect(pane).toHaveAttribute('data-outline-open', 'false')
  await expect(detailsToggle).not.toHaveClass(/text-primary/)
  await expect(detailsToggle.locator('svg')).toHaveClass(/lucide-panel-right-open/)
  await detailsToggle.click()
  await expect(pane).toHaveAttribute('data-outline-open', 'true')
  await expect(detailsToggle.locator('svg')).toHaveClass(/lucide-panel-right-close/)
  await expect(page.locator('.outline')).toContainText('Contents')
})

test('dashboard toggles the file explorer', async ({ page }) => {
  const pane = page.locator('.explorer-pane')
  const toggle = page.getByRole('button', { name: 'Toggle file explorer' })
  await expect(pane).toHaveAttribute('data-explorer-open', 'true')
  await expect(toggle).not.toHaveClass(/text-primary/)
  await toggle.click()
  await expect(pane).toHaveAttribute('data-explorer-open', 'false')
  await expect(toggle).not.toHaveClass(/text-primary/)
  await toggle.click()
  await expect(pane).toHaveAttribute('data-explorer-open', 'true')
  await expect(toggle).not.toHaveClass(/text-primary/)
})

test('dashboard toggles the application sidebar icon', async ({ page }) => {
  const toggle = page.getByRole('main').getByRole('button', { name: 'Toggle Sidebar' })
  await expect(toggle.locator('svg')).toHaveClass(/lucide-panel-left-close/)
  await toggle.click()
  await expect(toggle.locator('svg')).toHaveClass(/lucide-panel-left-open/)
  await toggle.click()
  await expect(toggle.locator('svg')).toHaveClass(/lucide-panel-left-close/)
})

test('dashboard resizes and remembers the file explorer width', async ({ page }) => {
  await page.evaluate(() => localStorage.removeItem('mybox_files_explorer_width'))
  await page.reload()

  const pane = page.locator('.explorer-pane')
  const handle = page.getByRole('separator', { name: 'Resize file explorer' })
  await expect(handle).toHaveAttribute('aria-valuenow', '280')

  const initialWidth = (await pane.boundingBox())!.width
  let handleBox = (await handle.boundingBox())!
  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(handleBox.x + handleBox.width / 2 + 100, handleBox.y + handleBox.height / 2)
  await page.mouse.up()
  await expect.poll(async () => (await pane.boundingBox())!.width).toBeGreaterThan(initialWidth + 90)
  await expect(handle).toHaveAttribute('aria-valuenow', '380')

  handleBox = (await handle.boundingBox())!
  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(handleBox.x + handleBox.width / 2 - 500, handleBox.y + handleBox.height / 2)
  await page.mouse.up()
  await expect(handle).toHaveAttribute('aria-valuenow', '180')

  handleBox = (await handle.boundingBox())!
  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(handleBox.x + handleBox.width / 2 + 500, handleBox.y + handleBox.height / 2)
  await page.mouse.up()
  await expect(handle).toHaveAttribute('aria-valuenow', '480')

  await page.reload()
  await expect(page.getByRole('separator', { name: 'Resize file explorer' })).toHaveAttribute('aria-valuenow', '480')
})

test('dashboard resizes and remembers the details width', async ({ page }) => {
  await closeAgentSidebar(page)
  await page.evaluate(() => localStorage.removeItem('mybox_files_details_width'))
  await page.reload()
  await treeButton(page.locator('.knowledge-explorer'), 'tasks.md').click()

  const pane = page.locator('.outline-pane')
  const handle = page.getByRole('separator', { name: 'Resize details' })
  await expect(handle).toHaveAttribute('aria-valuenow', '384')
  await expect(pane).toHaveAttribute('data-outline-open', 'true')
  await expect(pane).toBeVisible()

  const initialWidth = (await pane.boundingBox())!.width
  const handleBox = (await handle.boundingBox())!
  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(handleBox.x + handleBox.width / 2 - 100, handleBox.y + handleBox.height / 2)
  await page.mouse.up()

  await expect.poll(async () => (await pane.boundingBox())!.width).toBeGreaterThan(initialWidth + 90)
  await expect(handle).toHaveAttribute('aria-valuenow', '480')

  await handle.focus()
  await page.keyboard.press('Home')
  await expect(handle).toHaveAttribute('aria-valuenow', '180')
  await page.keyboard.press('End')
  await expect(handle).toHaveAttribute('aria-valuenow', '480')

  await page.reload()
  await expect(page.getByRole('separator', { name: 'Resize details' })).toHaveAttribute('aria-valuenow', '480')
})

test('dashboard resizes and remembers the application sidebar width', async ({ page }) => {
  await page.evaluate(() => localStorage.removeItem('mybox_sidebar_width'))
  await page.reload()

  const handle = page.getByRole('separator', { name: 'Resize sidebar' })
  await expect(handle).toHaveAttribute('aria-valuenow', '320')

  const handleBox = (await handle.boundingBox())!
  await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(handleBox.x + handleBox.width / 2 + 100, handleBox.y + handleBox.height / 2)
  await page.mouse.up()

  await expect(handle).toHaveAttribute('aria-valuenow', '420')
  await page.reload()
  await expect(page.getByRole('separator', { name: 'Resize sidebar' })).toHaveAttribute('aria-valuenow', '420')
})

test('nav bar file actions open a terminal', async ({ page }) => {
  await expect(page.getByRole('button', { name: 'New file' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()
  await page.getByRole('button', { name: 'Open terminal' }).click()
  await expect(page.locator('.terminal-panel')).toBeVisible()
})

test('New file menu creates files and folders from the Files tab', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  const newFileButton = page.getByRole('button', { name: 'New file', exact: true })

  await newFileButton.click()
  await expect(page.getByRole('menuitem', { name: 'New file', exact: true })).toBeVisible()
  await expect(page.getByRole('menuitem', { name: 'New folder', exact: true })).toBeVisible()

  await page.getByRole('menuitem', { name: 'New folder', exact: true }).click()
  await acceptAppDialog(page, 'created-from-new-folder-menu')
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/created-from-new-folder-menu$/)
  await expect(explorer).toContainText('created-from-new-folder-menu')

  await newFileButton.click()
  await page.getByRole('menuitem', { name: 'New file', exact: true }).click()
  await acceptAppDialog(page, 'created-from-new-file-menu.txt')
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/created-from-new-file-menu\.txt$/)
  await expect(explorer).toContainText('created-from-new-file-menu.txt')
})

test('terminal paste modal starts larger and supports horizontal resizing', async ({ page }) => {
  await page.getByRole('button', { name: 'Open terminal' }).click()
  await page.getByRole('button', { name: 'Paste' }).click()

  const input = page.getByPlaceholder('Tap here, then long-press → Paste')
  await expect(input).toHaveAttribute('rows', '6')
  await expect(input).toHaveCSS('resize', 'both')
  await expect(input).toHaveCSS('max-width', 'none')
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

test('the tab bar maximize button fills the main content area', async ({ page }) => {
  await page.getByRole('button', { name: 'Open terminal' }).click()
  const panel = page.locator('.terminal-panel')
  await expect(panel).toBeVisible()

  const rectBefore = await panel.boundingBox()
  expect(rectBefore).not.toBeNull()

  await page.getByRole('button', { name: 'Maximize terminal' }).click()
  const maximized = await panel.boundingBox()
  expect(maximized).not.toBeNull()
  const vw = await page.evaluate(() => ({ w: window.innerWidth, h: window.innerHeight }))
  const sidebar = await page.locator('[data-slot="sidebar-container"]').boundingBox()
  expect(sidebar).not.toBeNull()
  expect(maximized!.x).toBeGreaterThanOrEqual(sidebar!.width - 2)
  expect(maximized!.y).toBeLessThanOrEqual(2)
  expect(maximized!.x + maximized!.width).toBeGreaterThanOrEqual(vw.w - 2)
  expect(maximized!.height).toBeGreaterThanOrEqual(vw.h - 4)

  await page.getByRole('button', { name: 'Restore terminal' }).click()
  await expect(panel).toBeVisible()
  const restored = await panel.boundingBox()
  expect(restored).not.toBeNull()
  expect(restored!.height).toBeLessThan(vw.h - 4)
})

test('dragging the resize handle grows the internal terminal', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 1000 })
  await page.reload()
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

test('markdown editor completes link targets with project files', async ({ page }) => {
  await page.goto('/projects/proj/dashboard/files/knowledge/index.md')
  await page.getByRole('button', { name: 'Edit' }).click()
  const editor = page.locator('.monaco-editor', { has: page.getByLabel('File editor') })
  await editor.click()
  await page.keyboard.press('ControlOrMeta+A')
  await page.keyboard.insertText('')
  const rows = page.locator('.suggest-widget .monaco-list-row')

  // opening a link bracket-pair shows the project root as candidates
  await page.keyboard.type('[todo](')
  await expect(rows.filter({ hasText: 'docs/' })).toBeVisible()
  await expect(rows.filter({ hasText: 'README.md' })).toBeVisible()

  // typing further into a directory narrows to files and subdirectories
  await page.keyboard.type('./docs/gu')
  await expect(rows.filter({ hasText: 'guide.md' })).toBeVisible()

  // accepting inserts the full relative path
  await page.keyboard.press('Enter')
  await expect(editor).toContainText('./docs/guide.md')
})

test('markdown editor completes nested links after Tab-accepting a directory', async ({ page }) => {
  await page.goto('/projects/proj/dashboard/files/knowledge/index.md')
  await page.getByRole('button', { name: 'Edit' }).click()
  const editor = page.locator('.monaco-editor', { has: page.getByLabel('File editor') })
  await editor.click()
  await page.keyboard.press('ControlOrMeta+A')
  await page.keyboard.insertText('')
  const rows = page.locator('.suggest-widget .monaco-list-row')

  // Tab accepts the top suggestion (the docs/ directory, never the parent ..)
  await page.keyboard.type('[todo](')
  await expect(rows.filter({ hasText: 'docs/' })).toBeVisible()
  await page.keyboard.press('Tab')
  await expect(editor.locator('.view-line').first()).toContainText('./docs/')

  // accepting a directory re-triggers suggestions for its children
  await expect(rows.filter({ hasText: 'guide.md' })).toBeVisible()
  await expect(rows.filter({ hasText: 'recipes/' })).toBeVisible()

  // completing further into the directory still works
  await page.keyboard.type('gu')
  await expect(rows.filter({ hasText: 'guide.md' })).toBeVisible()
  await page.keyboard.press('Enter')
  await expect(editor).toContainText('./docs/guide.md')
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

  await page.getByTestId('file-tabs-favorites').click()
  const dialog = page.getByRole('dialog', { name: 'Favorites' })
  await expect(dialog).toContainText('tasks.md')
  await dialog.getByRole('button', { name: 'Remove proj: tasks.md from favorites' }).click()
  await expect(dialog).toContainText('No favorites yet.')
  await expect(page.getByRole('button', { name: '☆ Favorite' })).toBeVisible()
})

test('dashboard opens a favorite in its project from another project', async ({ page }) => {
  await page.goto('/projects/proj/dashboard/files/knowledge/docs/guide.md')
  await expect(page.getByText('Deep docs.')).toBeVisible()
  await page.getByRole('button', { name: '☆ Favorite' }).click()

  await page.goto('/projects/other/dashboard')
  await page.getByTestId('file-tabs-favorites').click()
  const dialog = page.getByRole('dialog', { name: 'Favorites' })
  await expect(dialog).toContainText('proj')
  await dialog.getByRole('button', { name: 'proj: knowledge/docs/guide.md', exact: true }).click()

  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/docs\/guide\.md$/)
  await expect(page.getByText('Deep docs.')).toBeVisible()
  await page.getByRole('button', { name: '★ Favorite' }).click()
  await expect(page.getByRole('button', { name: '☆ Favorite' })).toBeVisible()
})

test('dashboard records recently opened files', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await treeButton(explorer, 'tasks.md').click()
  await expect(page.locator('.file-tabs').getByRole('button', { name: 'tasks.md', exact: true })).toBeVisible()
})

test('dashboard moves a file by dragging onto a directory', async ({ page }) => {
  let moved = false
  try {
    const explorer = page.locator('.knowledge-explorer')
    await expect(explorer).toContainText('tasks.md')
    await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
    await treeButton(explorer, 'tasks.md')
      .dragTo(explorer.getByRole('button', { name: 'Collapse knowledge' }))
    moved = true
    await explorer.locator('.search-bar input').fill('knowledge/tasks')
    const list = explorer.locator('.file-list')
    await expect(list.getByRole('button', { name: 'knowledge/tasks.md' })).toBeVisible()
  } finally {
    if (moved) {
      const restored = await page.request.post('/api/files/move', {
        data: { old_path: 'knowledge/tasks.md', new_path: 'tasks.md' },
      })
      expect(restored.status()).toBe(204)
    }
  }
})

test('dashboard renames a file via Move', async ({ page }) => {
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks.md').click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/tasks\.md$/)
  await page.getByRole('button', { name: 'File actions' }).click()
  await page.getByRole('menuitem', { name: 'Move' }).click()
  await acceptAppDialog(page, 'knowledge/tasks2.md')
  await expect(treeButton(explorer, 'tasks2.md')).toBeVisible()
  await expect(explorer).toContainText('tasks2.md')
})

test('dashboard duplicates a file', async ({ page }) => {
  await closeAgentSidebar(page)
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await treeButton(explorer, 'tasks2.md').click()
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/tasks2\.md$/)
  await page.getByRole('button', { name: 'File actions' }).click()
  await page.getByRole('menuitem', { name: 'Duplicate' }).click()
  await acceptAppDialog(page, 'knowledge/tasks2-copy.md')
  // Scope to the tree: the opened viewer's meta line shows the same path.
  await expect(treeButton(explorer, 'tasks2-copy.md')).toBeVisible()
})

test('dashboard deletes a file', async ({ page }) => {
  let deleted = false
  try {
    await closeAgentSidebar(page)
    const explorer = page.locator('.knowledge-explorer')
    await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
    await treeButton(explorer, 'tasks2-copy.md').click()
    await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/tasks2-copy\.md$/)
    await page.getByRole('button', { name: 'File actions' }).click()
    await page.getByRole('menuitem', { name: 'Delete' }).click()
    await acceptAppDialog(page)
    deleted = true
    await expect(
      page.getByText('Select a file from the explorer to view it here.', { exact: true }),
    ).toBeVisible()
    await expect(explorer).not.toContainText('tasks2-copy.md')
  } finally {
    if (deleted) {
      const restored = await page.request.post('/api/files/move', {
        data: { old_path: 'knowledge/tasks2.md', new_path: 'tasks.md' },
      })
      expect(restored.status()).toBe(204)
    }
  }
})

test('dashboard executes an executable file and shows its output', async ({ page }) => {
  const projectsRes = await page.request.get('/api/projects')
  const projects = (await projectsRes.json()) as Array<{ name: string; path: string }>
  const proj = projects.find((p) => p.name === 'proj')
  expect(proj).toBeTruthy()
  const scripts = path.join(proj!.path, 'scripts')
  fs.mkdirSync(scripts, { recursive: true })
  const script = path.join(scripts, 'greet.sh')
  fs.writeFileSync(script, '#!/bin/sh\nprintf "hello from script %s\\n" mybox\n', { mode: 0o755 })

  await page.goto('/projects/proj/dashboard')
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'Expand scripts' }).click()
  const row = explorer.locator('.knowledge-tree-row', { hasText: 'greet.sh' })
  await expect(row.locator('.exec-file-badge')).toBeVisible()
  await row.click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Execute' }).click()

  const modal = page.getByRole('dialog')
  await expect(modal).toContainText('Exit code 0')
  await expect(modal.locator('pre')).toContainText('hello from script mybox')
  await modal.getByRole('button', { name: 'Close execution result' }).click()
  await expect(modal).toHaveCount(0)
})

test('dashboard keeps a running execution after closing and reloading its modal', async ({ page }) => {
  const projectsRes = await page.request.get('/api/projects')
  const projects = (await projectsRes.json()) as Array<{ name: string; path: string }>
  const proj = projects.find((p) => p.name === 'proj')
  expect(proj).toBeTruthy()
  const scripts = path.join(proj!.path, 'scripts')
  fs.mkdirSync(scripts, { recursive: true })
  const script = path.join(scripts, 'slow-greet.sh')
  fs.writeFileSync(
    script,
    '#!/bin/sh\nprintf "first line\\n"\nsleep 3\nprintf "second line\\n"\n',
    { mode: 0o755 },
  )

  try {
    await page.goto('/projects/proj/dashboard')
    await closeAgentSidebar(page)
    const explorer = page.locator('.knowledge-explorer')
    await explorer.getByRole('button', { name: 'Expand scripts' }).click()
    const row = explorer.locator('.knowledge-tree-row', { hasText: 'slow-greet.sh' })
    await row.click({ button: 'right' })
    await page.getByRole('menuitem', { name: 'Execute' }).click()

    const modal = page.getByRole('dialog', { name: 'Execute scripts/slow-greet.sh' })
    await expect(modal).toContainText('Running…')
    await modal.getByRole('button', { name: 'Close execution result' }).click()
    await expect(modal).toHaveCount(0)

    await page.reload()
    const executionStatus = page.getByRole('button', {
      name: /Open execution scripts\/slow-greet\.sh \(/,
    })
    await expect(executionStatus).toHaveCount(1)
    await executionStatus.click()
    await expect(page.getByRole('dialog', { name: 'Execute scripts/slow-greet.sh' })).toContainText('first line')
    await expect(page.getByRole('dialog', { name: 'Execute scripts/slow-greet.sh' })).toContainText('second line')
    await expect(page.getByRole('dialog', { name: 'Execute scripts/slow-greet.sh' })).toContainText('Exit code 0')
  } finally {
    fs.rmSync(script, { force: true })
  }
})

test('clicking the mybox brand returns to the unselected projects page', async ({ page }) => {
  await expect(page).toHaveURL(/\/projects\/proj\/dashboard(?:\/files\/.*)?$/)
  await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  await page.locator('.sidebar-nav').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page).toHaveURL(/\/board$/)
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  await page.getByRole('button', { name: 'Go to top' }).click()
  await expect(page.getByRole('heading', { name: 'Projects', level: 1 })).toBeVisible()
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

test('agent sidebar stays available across project tabs and remembers its open state', async ({ page }) => {
  const agentSidebar = page.getByTestId('agent-sidebar')
  await expect(agentSidebar).toBeVisible()
  await expect(agentSidebar).toContainText('proj')
  await expect(agentSidebar.getByRole('button', { name: 'Close agent sidebar' })).toHaveCount(0)

  await page.locator('.project-tabs').getByRole('link', { name: 'Board' }).click()
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  await expect(page.getByTestId('agent-sidebar')).toBeVisible()

  await page.getByTestId('agent-sidebar-toggle').click()
  await expect(page.getByTestId('agent-sidebar')).toHaveCount(0)
  await expect(page.getByTestId('agent-sidebar-collapsed')).toBeVisible()
  await expect(page.getByTestId('agent-sidebar-status-w7:p1')).toBeVisible()
  await page.getByTestId('agent-sidebar-toggle').click()
  await expect(page.getByTestId('agent-sidebar')).toBeVisible()
})

test.describe('narrow desktop project shell', () => {
  test.use({ viewport: { width: 768, height: 844 } })

  test('keeps the left sidebar inside the viewport when both sidebars are wide', async ({ page }) => {
    await page.evaluate(() => {
      localStorage.setItem('mybox_sidebar_width', '480')
      localStorage.setItem('mybox_agent_sidebar_width', '960')
    })
    await page.reload()

    await expect(page.getByTestId('agent-sidebar')).toBeVisible()
    await expect(page.getByRole('separator', { name: 'Resize agent sidebar' })).toHaveAttribute('aria-valuenow', '720')
    await expect.poll(async () => page.evaluate(() => ({
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      bodyWidth: document.body.scrollWidth,
    }))).toEqual({ viewportWidth: 768, documentWidth: 768, bodyWidth: 768 })

    const sidebar = page.locator('[data-slot="sidebar-container"]')
    await expect.poll(async () => (await sidebar.boundingBox())?.width ?? 0).toBeLessThanOrEqual(768)

    const agentSidebar = await page.getByTestId('agent-sidebar').boundingBox()
    const layoutWidth = await page.evaluate(() => document.documentElement.clientWidth)
    expect(agentSidebar).not.toBeNull()
    expect(agentSidebar!.x + agentSidebar!.width).toBeLessThanOrEqual(layoutWidth)
  })
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

test('herdr tab shows Herdr workspaces and operates agents', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  await expect(page.getByRole('heading', { name: 'Herdr', level: 1 })).toBeVisible()
  await openHerdrWorkspaces(page)
  await expect(page.getByTestId('herdr-workspace-w7')).toContainText('proj')
  await expect(page.getByTestId('herdr-workspace-w7')).toContainText('working')

  await page.route('**/api/herdr/agents/read', async (route) => {
    const request = route.request().postDataJSON() as { target?: string }
    if (request.target !== 'w7:p1') {
      await route.continue()
      return
    }
    const response = await route.fetch()
    const body = (await response.json()) as { output: string }
    await route.fulfill({
      response,
      json: { ...body, output: `${body.output}\n${'x'.repeat(2000)}` },
    })
  })

  await page.getByTestId('agent-sidebar-row-w7:p1').click()
  const detail = page.locator('.herdr-agent-detail')
  await expect(detail).toBeVisible()
  await expect(detail.locator('pre')).toContainText('stub output for w7:p1')
  const displayMode = detail.getByRole('combobox', { name: 'Agent output display mode' })
  await expect(displayMode).toHaveValue('auto')
  await displayMode.selectOption('herdr')
  await expect(detail.locator('pre')).toHaveClass(/whitespace-pre/)
  const outputPre = detail.locator('pre')
  await expect.poll(async () => outputPre.evaluate((element) => element.scrollWidth - element.clientWidth)).toBeGreaterThan(0)
  const herdrScroll = await outputPre.evaluate((element) => ({
    clientWidth: element.clientWidth,
    scrollWidth: element.scrollWidth,
  }))
  expect(herdrScroll.scrollWidth).toBeGreaterThan(herdrScroll.clientWidth)
  const herdrLayoutWidth = await page.evaluate(() => document.documentElement.clientWidth)
  await expect.poll(async () => page.evaluate(() => document.documentElement.scrollWidth)).toBe(herdrLayoutWidth)
  await displayMode.selectOption('auto')
  await expect(detail.locator('pre')).toHaveClass(/whitespace-pre-wrap/)
  const layoutWidth = await page.evaluate(() => document.documentElement.clientWidth)
  await expect.poll(async () => page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }))).toEqual({ clientWidth: layoutWidth, scrollWidth: layoutWidth })
  const agentSidebar = await page.getByTestId('agent-sidebar').boundingBox()
  const detailBox = await detail.boundingBox()
  expect(agentSidebar).not.toBeNull()
  expect(detailBox).not.toBeNull()
  expect(detailBox!.x + detailBox!.width).toBeLessThanOrEqual(agentSidebar!.x + agentSidebar!.width)

  // the focused agent polls its terminal output every second
  const pre = detail.locator('pre')
  const before = await pre.innerText()
  await expect(pre).not.toHaveText(before, { timeout: 5000 })

  const promptInput = page.getByTestId('herdr-prompt-input')
  await expect(promptInput).toHaveCSS('height', '24px')
  await promptInput.fill('run the tests please')
  const promptMetrics = await promptInput.evaluate((element) => ({
    clientHeight: element.clientHeight,
    scrollHeight: element.scrollHeight,
  }))
  expect(promptMetrics.scrollHeight).toBeLessThanOrEqual(promptMetrics.clientHeight)

  await page.locator('.project-tabs').getByRole('link', { name: 'Files' }).click()
  await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  await expect(page.getByTestId('herdr-prompt-input')).toHaveValue('run the tests please')

  const restoredPromptInput = page.getByTestId('herdr-prompt-input')
  await detail.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(detail.locator('.herdr-prompt-notice')).toContainText('prompt submitted')
  await expect(detail.locator('pre')).toContainText('last prompt: run the tests please')
  await expect(restoredPromptInput).toHaveValue('')
})

test('herdr tab and pane operations work end to end', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  await openHerdrWorkspaces(page)
  const ws = page.getByTestId('herdr-workspace-w7')
  await expect(ws).toBeVisible()
  await expect(page.getByTestId('herdr-tab-w7:t1')).toBeVisible()
  await expect(page.getByTestId('herdr-tab-w7:t2')).toContainText('2:')
  // the first tab is selected by default; focus is webui-managed
  await expect(page.getByTestId('herdr-tab-w7:t1')).toHaveAttribute('data-active', 'true')

  // create a tab via the custom prompt modal
  await ws.getByRole('button', { name: '+ New Tab' }).click()
  await acceptAppDialog(page, 'build')
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
  await openHerdrWorkspaces(page)
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
  await expect(p2Pre).toHaveText(frozen, { useInnerText: true })
  await toggle.click()
  await expect(toggle).toHaveAttribute('aria-pressed', 'true')

  // split an existing pane downward; the new pane appears under the same tab
  await p2.getByRole('button', { name: 'Split w7:p2 down' }).click()
  const p4 = page.locator('[data-testid^="herdr-pane-w7:p"]').filter({ hasText: 'stub-split-down' })
  await expect(p4).toBeVisible()

  // pane terminal output is loaded automatically while open
  await expect(page.locator('.herdr-pane-detail').locator('pre').first()).toContainText(
    'stub pane output for',
  )

  // rename the split pane
  await p4.getByRole('button', { name: 'Rename', exact: true }).click()
  await acceptAppDialog(page, 'logs')
  await expect(p4).toContainText('logs')

  // close the pane and then the tab (both confirm modals)
  await p4.locator('.herdr-pane-close').click()
  await acceptAppDialog(page)
  await expect(p4).toHaveCount(0)

  await newTab.hover()
  await newTab.getByRole('button', { name: `Close tab w7:t3` }).click()
  await acceptAppDialog(page)
  await expect(newTab).toHaveCount(0)
})

test('dragging a split divider resizes the neighboring panes', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  await openHerdrWorkspaces(page)
  const ws = page.getByTestId('herdr-workspace-w7')
  await expect(ws).toBeVisible()

  // the test owns a fresh tab so it starts from a single full pane
  await ws.getByRole('button', { name: '+ New Tab' }).click()
  await acceptAppDialog(page, 'drag')
  const tab = page.locator('[data-testid^="herdr-tab-w7:"]').filter({ hasText: 'drag' })
  await expect(tab).toContainText('drag')
  const tabId = (await tab.getAttribute('data-testid'))!.replace('herdr-tab-', '')
  await tab.click()
  await expect(tab).toHaveAttribute('data-active', 'true')

  const layout = ws.locator('.herdr-layout')
  await expect(layout).toBeVisible()
  const pane = layout.locator('[data-testid^="herdr-pane-"]').first()
  await expect(pane).toBeVisible()
  const paneId = (await pane.getAttribute('data-testid'))!.replace('herdr-pane-', '')

  // split it to the right so a vertical divider appears between the two panes
  await page.getByRole('button', { name: `Split ${paneId} right` }).click()
  const divider = layout.locator('[data-testid^="herdr-divider-"]').first()
  await expect(divider).toBeVisible()

  const before = (await pane.boundingBox())!.width
  const db = (await divider.boundingBox())!

  // drag the divider to the right; the left pane keeps the wider ratio
  await page.mouse.move(db.x + db.width / 2, db.y + db.height / 2)
  await page.mouse.down()
  await page.mouse.move(db.x + db.width / 2 + 120, db.y + db.height / 2, { steps: 24 })
  await page.mouse.up()
  await expect
    .poll(async () => (await pane.boundingBox())?.width, { timeout: 5000 })
    .toBeGreaterThan(before + 5)

  // the resize was committed to herdr: a reload keeps the wider split
  await page.reload()
  await openHerdrWorkspaces(page)
  await expect(layout).toBeVisible()
  const paneAfter = layout.locator('[data-testid^="herdr-pane-"]').first()
  await expect(paneAfter).toBeVisible()
  await expect
    .poll(async () => (await paneAfter.boundingBox())?.width, { timeout: 5000 })
    .toBeGreaterThan(before + 5)

  // clean up the tab so later tests start from a predictable server state
  await tab.hover()
  await tab.getByRole('button', { name: `Close tab ${tabId}` }).click()
  await acceptAppDialog(page)
  await expect(tab).toHaveCount(0)
})

test('sidebar shows Herdr workspace status and agents', async ({ page }) => {
  await page.goto('/projects/proj/dashboard')
  const projRow = page
    .locator('.sidebar-projects button')
    .filter({ hasText: 'proj' })
    .first()
  await expect(
    projRow.locator('.herdr-workspace-status[aria-label="Herdr workspace status working"]'),
  ).toBeVisible()

  const agents = page.locator('.sidebar-agents')
  await expect(agents.locator('[aria-label="agent status working"]').first()).toBeVisible()
  // agent row details: workspace label, cwd directory name and terminal title
  const row = page.getByTestId('sidebar-agent-w7:p1')
  await expect(row).toContainText('proj')
  await expect(row).toContainText('proj-dir')
  await expect(row).toContainText('OC | stub agent')
  await expect(page.getByTestId('sidebar-agent-w7:p2')).toHaveCount(0)
})

test('clicking a sidebar agent opens its operation panel in the herdr tab', async ({ page }) => {
  await page.goto('/projects/proj/dashboard')
  await page.getByTestId('sidebar-agent-w7:p1').click()
  await expect(page).toHaveURL(/\/projects\/proj\/herdr\?agent=w7%3Ap1$/)
  const detail = page.locator('.herdr-agent-detail')
  await expect(detail).toBeVisible()
  await expect(detail.locator('pre')).toContainText('stub output for w7:p1')

  await page.getByTestId('herdr-prompt-input').fill('hello from sidebar')
  await detail.getByRole('button', { name: 'Send', exact: true }).click()
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

test('clicking a sidebar agent linked to a task opens its file in the Files tab', async ({
  page,
}) => {
  // Give the project agent the name herdr would assign to a task directory, so
  // the sidebar can link it back to that task's file.
  await page.route('**/api/herdr/overview', async (route) => {
    const res = await route.fetch()
    const body = await res.json()
    body.agents = body.agents.map((a: { pane_id: string }) =>
      a.pane_id === 'w7:p1'
        ? { ...a, name: 'e2e-status-change-target', custom_name: 'e2e-status-change-target' }
        : a,
    )
    body.tabs = body.tabs.map((tab: { tab_id: string }) =>
      tab.tab_id === 'w7:t1' ? { ...tab, label: 'e2e-status-change-target' } : tab,
    )
    await route.fulfill({ response: res, json: body })
  })
  await page.goto('/projects/proj/dashboard')
  await page.getByTestId('sidebar-agent-w7:p1').click()
  await expect(page).toHaveURL(
    /\/projects\/proj\/dashboard\/files\/_tasks\/e2e-status-change-target\/task\.md$/,
  )
  await expect(page.getByRole('heading', { name: 'Drag me to done' })).toBeVisible()
  await page.unrouteAll({ behavior: 'ignoreErrors' })
})

test('herdr offers a new tab when no tabs or panes exist at all', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  await openHerdrWorkspaces(page)

  // close every visible tab; the last tab of a workspace also removes the
  // workspace. (Herdr workspaces of other projects are not shown on this page and
  // do not count as tabs/panes of this project.)
  for (const tabId of ['w7:t1', 'w7:t2']) {
    const tab = page.getByTestId(`herdr-tab-${tabId}`)
    await tab.hover()
    await tab.getByRole('button', { name: `Close tab ${tabId}` }).click()
    await acceptAppDialog(page)
    await expect(tab).toHaveCount(0)
  }

  // with nothing left, the empty state still offers tab creation
  const empty = page.getByTestId('herdr-no-workspace')
  await expect(empty).toContainText(`No herdr workspace found for project "proj"`)
  await expect(page.getByTestId('herdr-create-first-tab')).toBeVisible()

  await page.getByTestId('herdr-create-first-tab').click()
  await acceptAppDialog(page, 'fresh')

  // bootstrapping creates the project's first workspace with the named tab
  const workspace = page.locator('[data-testid^="herdr-workspace-"]')
  await expect(workspace).toHaveCount(1)
  await expect(workspace).toContainText('proj')
  await expect(page.locator('[data-testid^="herdr-tab-"]').filter({ hasText: 'fresh' })).toBeVisible()
})

test('graph tab shows the explorer and graph view', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()

  const explorer = page.locator('.explorer-pane')
  await expect(explorer.getByText('README.md').first()).toBeVisible()

  const canvas = page.locator('.graph-canvas')
  await expect(canvas).toBeVisible()
  await expect(page.getByText(/nodes · .* edges/)).toBeVisible()

  // expanding a directory surfaces its children in both explorer and graph
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await expect(explorer.getByRole('button', { name: 'Collapse knowledge' }).first()).toBeVisible()
  await expect(explorer.getByText('index.md').first()).toBeVisible()
})

test('graph explores a file by double-clicking it in the explorer', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  const explorer = page.locator('.explorer-pane')
  await expect(explorer.getByText('README.md').first()).toBeVisible()
  await explorer.getByText('README.md').first().dblclick()
  await expect(page).toHaveURL(/\/dashboard\/files\/README\.md$/)
  await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
})

test('graph restores the previous explorer state when returning to the tab', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()

  const explorer = page.locator('.explorer-pane')
  await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  await expect(explorer.getByRole('button', { name: 'Collapse knowledge' }).first()).toBeVisible()

  // switch away and back
  await page.locator('.project-tabs').getByRole('link', { name: 'Files' }).click()
  await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()

  // the expanded directory is still expanded, its file still listed
  await expect(explorer.getByRole('button', { name: 'Collapse knowledge' }).first()).toBeVisible()
  await expect(explorer.getByText('index.md').first()).toBeVisible()
})

test('explorer right-click expands and collapses a directory recursively', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  const explorer = page.locator('.explorer-pane')
  await expect(explorer.getByText('knowledge', { exact: true }).first()).toBeVisible()

  // deep files are hidden until the directory is expanded recursively
  await expect(explorer.getByText('guide.md').first()).toHaveCount(0)
  await expect(explorer.getByText('pizza.md').first()).toHaveCount(0)

  await explorer.getByText('knowledge', { exact: true }).first().click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Expand all' }).click()

  await expect(explorer.getByText('guide.md').first()).toBeVisible()
  await expect(explorer.getByText('pizza.md').first()).toBeVisible()

  // collapse-all folds the whole subtree away again
  await explorer.getByText('knowledge', { exact: true }).first().click({ button: 'right' })
  await page.getByRole('menuitem', { name: 'Collapse all' }).click()

  await expect(explorer.getByText('guide.md').first()).toHaveCount(0)
  await expect(explorer.getByText('pizza.md').first()).toHaveCount(0)
})

test('board drag-and-drop changes task status and front matter', async ({ page }) => {
  await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()

  await expect(page.getByRole('button', { name: 'New task' })).toBeVisible()

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

test('task without type is created and listed plainly, and can be archived', async ({ page }) => {
  const created = await page.request.post('/api/tasks', {
    headers: { 'X-Project': 'proj' },
    data: { name: 'Review PR #99', priority: 'urgent' },
  })
  expect(created.ok()).toBeTruthy()
  const task = (await created.json()) as { id: string; type?: string }
  expect(task.type).toBeUndefined()

  await page.goto('/projects/proj/dashboard')
  await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  await expect(page.getByText('Review PR #99')).toBeVisible()
  const card = page.locator('.board-card').filter({ hasText: 'Review PR #99' })
  await expect(card).not.toHaveClass(/adhoc-card/)
  await expect(card.locator('.badge', { hasText: 'adhoc' })).toHaveCount(0)

  const archived = await page.request.post(`/api/tasks/${task.id}/archive`, { headers: { 'X-Project': 'proj' } })
  expect(archived.status()).toBe(204)

  const res = await page.request.get('/api/tasks', { headers: { 'X-Project': 'proj' } })
  const tasks = (await res.json()) as Array<{ id: string }>
  expect(tasks.some((t) => t.id === task.id)).toBeFalsy()
})

test.describe('mobile viewport', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test.beforeEach(async ({ page }) => {
    await page.goto('/projects/proj/dashboard/files/tasks.md')
    await closeAgentSidebar(page)
  })

  test('hides the details sidebar by default and opens it as a slide-over', async ({ page }) => {
    await expect(page.locator('.outline')).toHaveCount(0)
    await page.getByRole('button', { name: 'Toggle details' }).click()
    await expect(page.locator('.outline')).toContainText('Contents')
    await page.getByRole('button', { name: 'Close' }).click()
    await expect(page.locator('.outline')).toHaveCount(0)
  })

  test('keeps the closed agent sidebar out of the layout and shows its status in the header', async ({ page }) => {
    await expect(page.getByTestId('agent-sidebar')).toHaveCount(0)
    await expect(page.getByTestId('agent-sidebar-collapsed')).toHaveCount(0)

    const agentToggle = page.getByTestId('agent-sidebar-toggle')
    await expect(agentToggle).toHaveAttribute('aria-label', /^Agents:.*; Open agent sidebar$/)
    await expect(agentToggle.locator('[role="img"]')).toBeVisible()
    await expect.poll(async () => page.evaluate(() => ({
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      bodyWidth: document.body.scrollWidth,
    }))).toEqual({ viewportWidth: 390, documentWidth: 390, bodyWidth: 390 })

    await agentToggle.click()
    await expect(page.getByTestId('agent-sidebar-sheet')).toBeVisible()
    await expect.poll(async () => page.evaluate(() => ({
      viewportWidth: window.innerWidth,
      documentWidth: document.documentElement.scrollWidth,
      bodyWidth: document.body.scrollWidth,
    }))).toEqual({ viewportWidth: 390, documentWidth: 390, bodyWidth: 390 })
    await closeAgentSidebar(page)
  })

  test('grows the agent prompt input as text wraps on mobile', async ({ page }) => {
    await page.getByTestId('agent-sidebar-toggle').click()
    await expect(page.getByTestId('agent-sidebar-sheet')).toBeVisible()
    const agentRow = page.locator('[data-testid^="agent-sidebar-row-"]').first()
    await expect(agentRow).toBeVisible()
    await agentRow.click()

    const promptInput = page.getByTestId('herdr-prompt-input')
    await expect(promptInput).toHaveCSS('height', '48px')
    await promptInput.fill('This is a long prompt that should wrap across multiple lines on a narrow phone viewport. '.repeat(5))
    await expect.poll(async () => promptInput.evaluate((element) => element.clientHeight)).toBeGreaterThan(48)

    await promptInput.fill('short')
    await expect(promptInput).toHaveCSS('height', '48px')
    await closeAgentSidebar(page)
  })

  test('groups secondary agent controls in a mobile command palette', async ({ page }) => {
    await page.getByTestId('agent-sidebar-toggle').click()
    await expect(page.getByTestId('agent-sidebar-sheet')).toBeVisible()
    const agentRow = page.locator('[data-testid^="agent-sidebar-row-"]').first()
    await expect(agentRow).toBeVisible()
    await agentRow.click()
    const paneId = (await agentRow.getAttribute('data-testid'))!.replace('agent-sidebar-row-', '')

    await expect(page.getByTestId(`agent-key-${paneId}-Enter`)).toBeVisible()
    await expect(page.getByTestId(`agent-key-${paneId}-↑`)).toBeVisible()
    await expect(page.getByTestId(`agent-key-${paneId}-↓`)).toBeVisible()
    await expect(page.getByTestId(`agent-key-${paneId}-Esc`)).toHaveCount(0)
    await page.getByTestId('agent-command-palette-toggle').click()

    const palette = page.getByTestId('agent-command-palette')
    await expect(palette).toBeVisible()
    await expect(page.getByTestId('agent-command-palette-commands')).toHaveClass(/grid-cols-2/)
    await expect(page.getByTestId(`agent-key-${paneId}-Esc`)).toBeVisible()
    await expect(palette.getByTestId(`agent-key-${paneId}-↑`)).toHaveCount(0)
    await expect(palette.getByTestId(`agent-key-${paneId}-↓`)).toHaveCount(0)
    await expect(page.getByTestId(`agent-command-${paneId}-status`)).toHaveAttribute('data-size', 'xs')
    await expect(palette.getByRole('searchbox', { name: 'Filter keys and commands' })).toHaveCount(0)
    await page.getByTestId(`agent-command-${paneId}-status`).click()
    await expect(palette).toHaveCount(0)
    await closeAgentSidebar(page)
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
  test.beforeEach(async ({ page }) => {
    // Git tests share the server's project fixture. Start each case without
    // repository state so their setup does not depend on execution order.
    await removeProjectGitRepo(page)
  })

  test('resizes and remembers the git explorer width', async ({ page }) => {
    await page.evaluate(() => localStorage.removeItem('mybox_git_explorer_width'))
    await page.goto('/projects/proj/git')
    const noRepo = page.getByRole('heading', { name: 'No git repository' })
    try {
      await noRepo.waitFor({ state: 'visible', timeout: 2000 })
      await page.getByRole('button', { name: 'Initialize repository' }).click()
      await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()
    } catch {
      // The repository already exists from another Git test.
    }

    const pane = page.locator('.explorer-pane')
    const handle = page.getByRole('separator', { name: 'Resize git explorer' })
    await expect(handle).toHaveAttribute('aria-valuenow', '280')

    const initialWidth = (await pane.boundingBox())!.width
    const handleBox = (await handle.boundingBox())!
    await page.mouse.move(handleBox.x + handleBox.width / 2, handleBox.y + handleBox.height / 2)
    await page.mouse.down()
    await page.mouse.move(handleBox.x + handleBox.width / 2 + 100, handleBox.y + handleBox.height / 2)
    await page.mouse.up()

    await expect.poll(async () => (await pane.boundingBox())!.width).toBeGreaterThan(initialWidth + 90)
    await expect(handle).toHaveAttribute('aria-valuenow', '380')

    await page.reload()
    await expect(page.getByRole('separator', { name: 'Resize git explorer' })).toHaveAttribute('aria-valuenow', '380')
  })

  test('initializes a repository, lists files, and commits the project', async ({ page }) => {
    await page.goto('/projects/proj/git')
    await expect(page.getByRole('heading', { name: 'No git repository' })).toBeVisible()
    await page.getByRole('button', { name: 'Initialize repository' }).click()
    await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()

    // Seed files are now visible as untracked (basenames; README.md appears
    // at the root and under knowledge/).
    await expect(page.locator('.knowledge-tree').getByRole('button', { name: 'README.md' })).toHaveCount(2)
    await expect(page.locator('.knowledge-tree').getByRole('button', { name: 'index.md' })).toHaveCount(1)

    // The commit box is disabled until a message is entered; committing with
    // "Stage all before commit" enabled captures the whole project.
    const commit = page.getByRole('button', { name: 'Commit', exact: true })
    await expect(commit).toBeDisabled()
    const commitMessage = page.getByTestId('git-commit-message')
    await commitMessage.fill('Initial commit')
    await expect(commit).toBeEnabled()
    await commitMessage.press('Control+Enter')
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

    await page.getByTestId('git-commit-message').fill('amended subject')
    await page.getByRole('button', { name: 'Amend' }).click()
    await acceptAppDialog(page)
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

  test('opens the branch menu within the mobile explorer viewport', async ({ page }) => {
    await page.goto('/projects/proj/git')
    await closeAgentSidebar(page)
    const noRepo = page.getByRole('heading', { name: 'No git repository' })
    try {
      await noRepo.waitFor({ state: 'visible', timeout: 2000 })
      await page.getByRole('button', { name: 'Initialize repository' }).click()
      await expect(page.getByRole('heading', { name: 'Git', level: 1 })).toBeVisible()
    } catch {
      // The repository already exists from another Git test.
    }

    await page.getByRole('button', { name: 'Toggle file explorer' }).click()
    const branchSwitcher = page.getByTestId('git-explorer-branch-switcher')
    await expect(branchSwitcher).toBeVisible()
    await branchSwitcher.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()
    await expect.poll(async () => (await menu.boundingBox())?.y ?? -1).toBeGreaterThanOrEqual(0)
  })

  test('working tree opens as a slide-over and stays reachable after selecting a file', async ({ page }) => {
    await page.request.post('/api/files', { data: { path: 'tmp-untracked.md' } })
    try {
      await page.goto('/projects/proj/git')
      await closeAgentSidebar(page)
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

  test('shows projects with an unselected project box and a Projects-only menu', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible()
    const nav = page.locator('.sidebar-nav')
    await expect(nav.getByRole('link', { name: 'Projects' })).toBeVisible()
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

    await row.getByRole('button', { name: 'Delete' }).click()
    await acceptAppDialog(page)
    await expect(
      page.locator('.projects-table tbody tr', { hasText: path.basename(dir) }),
    ).toBeHidden()
  })
})
