import { test, expect } from '@playwright/test'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

test.beforeEach(async ({ page }) => {
  await page.goto('/proj/')
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

test('dashboard shows a status badge for markdown files with status metadata', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  const row = explorer.locator('.knowledge-tree-row', { hasText: 'task.md' })
  await expect(row.locator('.badge.status-doing')).toBeVisible()
  await expect(row.locator('.badge.status-doing')).toHaveText('doing')
})

test('dashboard opens a markdown file from the explorer', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await expect(explorer).toContainText('tasks.md')
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  await expect(
    explorer.getByRole('button', { name: 'tasks.md', exact: true }),
  ).toHaveClass(/active/)
})

test('dashboard keeps the open file in the URL across a reload', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  await expect(page).toHaveURL(/#\/files\/tasks\.md$/)
  await page.reload()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  await expect(
    explorer.getByRole('button', { name: 'tasks.md', exact: true }),
  ).toHaveClass(/active/)
})

test('dashboard shows a content outline for markdown files', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
  await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  const outline = page.locator('.outline')
  await expect(outline).toContainText('Content')
  await expect(outline.getByRole('link', { name: 'Tasks' })).toBeVisible()
})

test('dashboard edits and saves a file', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
  await page.getByRole('button', { name: 'Edit' }).click()
  const editor = page.getByLabel('File editor')
  await editor.fill('# Task tracking\n\nEdited content.\n')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Saved.')).toBeVisible()
  await expect(page.getByText('Edited content.')).toBeVisible()
})

test('dashboard edits frontmatter metadata separately from the body', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
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
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
  await page.getByRole('button', { name: '☆ Favorite' }).click()
  await expect(page.getByRole('button', { name: '★ Favorite' })).toBeVisible()
  const favorites = page.locator('.sidebar-section').filter({ hasText: 'Favorites' })
  await expect(favorites.locator('.sidebar-list')).toContainText('tasks.md')
})

test('dashboard moves a file by dragging onto a directory', async ({ page }) => {
  const explorer = page.locator('.knowledge-explorer')
  await expect(explorer).toContainText('tasks.md')
  await explorer
    .getByRole('button', { name: 'tasks.md', exact: true })
    .dragTo(explorer.getByRole('button', { name: 'Collapse knowledge' }))
  await explorer.locator('.search-bar input').fill('knowledge/tasks')
  const list = explorer.locator('.file-list')
  await expect(list.getByRole('button', { name: 'knowledge/tasks.md' })).toBeVisible()
})

test('dashboard renames a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept('tasks2.md'))
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks.md', exact: true }).click()
  await page.getByRole('button', { name: 'Rename' }).click()
  await expect(page.getByText('knowledge/tasks2.md', { exact: true })).toBeVisible()
  await expect(explorer).toContainText('tasks2.md')
})

test('dashboard duplicates a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept('knowledge/tasks2-copy.md'))
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks2.md', exact: true }).click()
  await page.getByRole('button', { name: 'Duplicate' }).click()
  await expect(page.getByText('knowledge/tasks2-copy.md', { exact: true })).toBeVisible()
})

test('dashboard deletes a file', async ({ page }) => {
  page.on('dialog', (d) => d.accept())
  const explorer = page.locator('.knowledge-explorer')
  await explorer.getByRole('button', { name: 'tasks2-copy.md', exact: true }).click()
  await page.getByRole('button', { name: 'Delete' }).click()
  await expect(
    page.getByText('Select a file from the explorer to view it here.', { exact: true }),
  ).toBeVisible()
  await expect(explorer).not.toContainText('tasks2-copy.md')
})

test('task list shows seeded tasks', async ({ page }) => {
  await page.getByRole('link', { name: 'Tasks' }).click()
  await expect(page.getByRole('heading', { name: 'Tasks' })).toBeVisible()
  await expect(page.getByText('Ship the web UI')).toBeVisible()
  await expect(page.getByText('Write E2E tests')).toBeVisible()
})

test('clicking the mybox brand returns to the unselected projects page', async ({ page }) => {
  await page.getByRole('link', { name: 'Tasks' }).click()
  await expect(page.getByRole('heading', { name: 'Tasks' })).toBeVisible()
  await page.getByRole('button', { name: 'Go to top' }).click()
  await expect(page.getByRole('heading', { name: 'Projects', level: 1 })).toBeVisible()
  await expect(page.locator('.sidebar-brand select')).toHaveValue('')
  await expect(page.locator('.sidebar-nav').getByRole('link', { name: 'Dashboard' })).toBeHidden()
})

test('edits a task status and assignee from the detail page', async ({ page }) => {
  await page.getByRole('link', { name: 'Tasks' }).click()
  await page.getByRole('button', { name: 'Ship the web UI' }).click()
  await expect(page.getByRole('heading', { name: 'Ship the web UI' }).first()).toBeVisible()

  await page.getByRole('button', { name: 'Edit' }).click()
  await page.getByLabel('Status').selectOption('done')
  await page.getByLabel('Assignee').fill('alice')
  await page.getByRole('button', { name: 'Save' }).click()

  await expect(page.locator('.badge.status-done')).toBeVisible()
  await expect(page.getByText('assignee: alice')).toBeVisible()

  const res = await page.request.get('/api/tasks')
  const tasks = (await res.json()) as Array<{ id: string; title: string; status: string; assignee: string }>
  const edited = tasks.find((t) => t.title === 'Ship the web UI')
  expect(edited?.status).toBe('done')
  expect(edited?.assignee).toBe('alice')
})

test('creates and opens a task', async ({ page }) => {
  await page.getByRole('link', { name: 'Tasks' }).click()
  page.once('dialog', (d) => d.accept('Newly created task'))
  await page.getByRole('button', { name: 'New task' }).click()
  await expect(
    page.getByRole('heading', { name: 'Newly created task' }).first(),
  ).toBeVisible()
})

test('knowledge view renders markdown with wiki links', async ({ page }) => {
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'index', exact: true }).click()
  await expect(page.getByText('Welcome to the workspace.')).toBeVisible()
})

test('search finds knowledge and opens it', async ({ page }) => {
  await page.getByPlaceholder('Search…').first().fill('phase6')
  await page.getByPlaceholder('Search…').first().press('Enter')
  await expect(page.getByRole('heading', { name: 'Search' })).toBeVisible()
  await page.getByRole('button', { name: 'Phase 6' }).click()
  await expect(page.getByText('The HTTP API is done.')).toBeVisible()
})

test('knowledge view shows outline and renders mermaid', async ({ page }) => {
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'phase6', exact: true }).click()
  await expect(page.getByText('Overview', { exact: true }).first()).toBeVisible()
  await expect(page.locator('.outline')).toContainText('Overview')
  await expect(page.locator('.outline')).toContainText('Diagram')
  await expect(page.locator('.mermaid svg')).toBeVisible()
})

test('knowledge outline link scrolls to its heading', async ({ page }) => {
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'phase6', exact: true }).click()
  await expect(page.locator('.outline')).toContainText('Diagram')
  await expect
    .poll(() => page.locator('.outline').evaluate((el) => getComputedStyle(el).position))
    .toBe('sticky')
  await page.getByRole('link', { name: 'Diagram' }).click()
  await expect(page.locator('h2#diagram')).toBeInViewport()
})

test('knowledge page switches explorer/viewer on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('button', { name: 'Toggle menu' }).click()
  await page.getByRole('link', { name: 'Knowledge' }).click()

  await expect(page.locator('.knowledge-explorer')).toBeVisible()
  await expect(page.locator('.knowledge-pane')).toBeHidden()

  await page.locator('main').getByRole('button', { name: 'phase6', exact: true }).click()
  await expect(page.locator('.knowledge-explorer')).toBeHidden()
  await expect(page.locator('.knowledge-pane')).toBeVisible()
  await expect(page.getByRole('button', { name: '← Files' })).toBeVisible()

  await page.getByRole('button', { name: '← Files' }).click()
  await expect(page.locator('.knowledge-explorer')).toBeVisible()
  await expect(page.locator('.knowledge-pane')).toBeHidden()
})

test('edits a knowledge file and persists to disk', async ({ page }) => {
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'phase6', exact: true }).click()

  await page.getByRole('button', { name: 'Edit' }).click()
  const editor = page.getByLabel('Markdown editor')
  await editor.fill('# Phase 6\n\n## Overview\n\nEdited via E2E.\n')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Edited via E2E.')).toBeVisible()

  await page.reload()
  await expect(page.getByText('Edited via E2E.')).toBeVisible()
})

test('graph renders knowledge nodes', async ({ page }) => {
  await page.getByRole('link', { name: 'Graph' }).click()
  await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
})

test('board drag-and-drop changes task status and front matter', async ({ page }) => {
  await page.getByRole('link', { name: 'Board', exact: true }).click()
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

test('wiki links resolve by path, title, and alias', async ({ page }) => {
  await page.request.put('/api/knowledge/content', {
    data: {
      path: 'notes/phase6',
      content: '---\ntitle: Phase 6\naliases: [P6]\n---\n\n# Phase 6\n',
    },
  })
  await page.request.put('/api/knowledge/content', {
    data: {
      path: 'notes/wiki-test',
      content:
        'by path: [[notes/phase6]]\n' +
        'by title: [[Phase 6]]\n' +
        'by alias: [[P6]]\n' +
        'by basename: [[phase6]]\n',
    },
  })
  await page.goto('/proj/#/knowledge/notes/wiki-test')
  await expect(page.getByText('by path:')).toBeVisible()
  const hrefs = await page
    .locator('.markdown-body a')
    .evaluateAll((as) => as.map((a) => a.getAttribute('href')))
  expect(hrefs).toEqual([
    '#/knowledge/notes%2Fphase6',
    '#/knowledge/notes%2Fphase6',
    '#/knowledge/notes%2Fphase6',
    '#/knowledge/notes%2Fphase6',
  ])
})

test('relative markdown links resolve against the note directory', async ({ page }) => {
  await page.request.put('/api/knowledge/content', {
    data: { path: 'golang/golang_project_structure', content: '# Structure\n' },
  })
  await page.request.put('/api/knowledge/content', {
    data: {
      path: 'golang/golang_architecture',
      content: 'see [structure](./golang_project_structure.md)\nand [top](../index.md)\n',
    },
  })
  await page.goto('/proj/#/knowledge/golang/golang_architecture')
  const link = page.locator('.markdown-body a', { hasText: 'structure' })
  await expect(link).toHaveAttribute('href', '#/knowledge/golang%2Fgolang_project_structure')
  const top = page.locator('.markdown-body a', { hasText: 'top' })
  await expect(top).toHaveAttribute('href', '#/knowledge/index')
  await link.click()
  await expect(
    page.locator('.markdown-body').getByRole('heading', { name: 'Structure' }),
  ).toBeVisible()
})

test('outline does not accumulate when navigating between notes', async ({ page }) => {
  await page.request.put('/api/knowledge/content', {
    data: {
      path: 'notes/dup-a',
      content: '# Note A\n\n## Library\n\n## Library\n\n## Reason\n\n## Reason\n\nsee [[notes/dup-b]]\n',
    },
  })
  await page.request.put('/api/knowledge/content', {
    data: {
      path: 'notes/dup-b',
      content: '# Note B\n\n## Overview\n\n## Overview\n\nsee [[notes/dup-a]]\n',
    },
  })
  await page.goto('/proj/#/knowledge/notes/dup-a')
  await expect(page.locator('.outline-link')).toHaveCount(5)
  await page.locator('.markdown-body a', { hasText: 'notes/dup-b' }).click()
  await expect(page.locator('.outline-link')).toHaveCount(3)
  await page.locator('.markdown-body a', { hasText: 'notes/dup-a' }).click()
  await expect(page.locator('.outline-link')).toHaveCount(5)
})

test('outline shows a graph of linked notes', async ({ page }) => {
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'index', exact: true }).click()
  const block = page.locator('.outline-section').filter({ hasText: 'Graph' })
  await expect(block.getByText('Graph')).toBeVisible()
  await expect(block.locator('canvas')).toBeVisible()
})

test.describe('project selection at /', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/')
  })

  test('shows projects with an unselected project box and a Projects-only menu', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible()
    const nav = page.locator('.sidebar-nav')
    await expect(nav.getByRole('link', { name: 'Projects' })).toBeVisible()
    await expect(nav.getByRole('link', { name: 'Dashboard' })).toBeHidden()
    await expect(page.locator('.sidebar-brand select')).toHaveValue('')
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
