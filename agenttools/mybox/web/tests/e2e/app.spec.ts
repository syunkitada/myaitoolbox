import { test, expect } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.goto('/')
})

test('renders dashboard with stats', async ({ page }) => {
  await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()
  await expect(page.getByText('open tasks', { exact: true })).toBeVisible()
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
  await page.getByRole('button', { name: 'index', exact: true }).click()
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
  await page.locator('main').getByRole('button', { name: 'notes/phase6', exact: true }).click()
  await expect(page.getByText('Overview', { exact: true }).first()).toBeVisible()
  await expect(page.locator('.outline')).toContainText('Overview')
  await expect(page.locator('.outline')).toContainText('Diagram')
  await expect(page.locator('.mermaid svg')).toBeVisible()
})

test('edits a knowledge file and persists to disk', async ({ page }) => {
  await page.getByRole('link', { name: 'Knowledge' }).click()
  await page.locator('main').getByRole('button', { name: 'notes/phase6', exact: true }).click()

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
