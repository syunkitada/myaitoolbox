import { test, expect } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.goto('/')
})

test('dashboard shows project file explorer with README by default', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'Files' })).toBeVisible()
  await expect(
    page.getByText('This project tracks tasks and knowledge.', { exact: true }),
  ).toBeVisible()
  const explorer = page.locator('.knowledge-explorer')
  await expect(explorer).toContainText('README.md')
  await expect(explorer).toContainText('knowledge')
  await expect(explorer).toContainText('tasks')
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

test('task list shows seeded tasks', async ({ page }) => {
  await page.getByRole('link', { name: 'Tasks' }).click()
  await expect(page.getByRole('heading', { name: 'Tasks' })).toBeVisible()
  await expect(page.getByText('Ship the web UI')).toBeVisible()
  await expect(page.getByText('Write E2E tests')).toBeVisible()
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
  await page.goto('/#/knowledge/notes/wiki-test')
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
  await page.goto('/#/knowledge/golang/golang_architecture')
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
  await page.goto('/#/knowledge/notes/dup-a')
  await expect(page.locator('.outline-link')).toHaveCount(5)
  await page.locator('.markdown-body a', { hasText: 'notes/dup-b' }).click()
  await expect(page.locator('.outline-link')).toHaveCount(3)
  await page.locator('.markdown-body a', { hasText: 'notes/dup-a' }).click()
  await expect(page.locator('.outline-link')).toHaveCount(5)
})

test('outline shows a graph of linked notes', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'index', exact: true }).click()
  const block = page.locator('.outline-section').filter({ hasText: 'Graph' })
  await expect(block.getByText('Graph')).toBeVisible()
  await expect(block.locator('canvas')).toBeVisible()
})
