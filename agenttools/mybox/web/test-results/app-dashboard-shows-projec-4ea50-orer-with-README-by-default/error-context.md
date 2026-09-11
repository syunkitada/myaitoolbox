# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: app.spec.ts >> dashboard shows project file explorer with README by default
- Location: tests/e2e/app.spec.ts:41:1

# Error details

```
Error: expect(locator).toBeVisible() failed

Locator: getByText('This project tracks tasks and knowledge.', { exact: true })
Expected: visible
Timeout: 5000ms
Error: element(s) not found

Call log:
  - Expect "toBeVisible" with timeout 5000ms
  - waiting for getByText('This project tracks tasks and knowledge.', { exact: true })

```

```yaml
- list:
  - listitem:
    - button "Go to top": mybox
- list:
  - listitem:
    - link "Workspaces":
      - /url: /projects
  - listitem:
    - link "Board":
      - /url: /board
  - listitem:
    - link "Stats":
      - /url: /stats
- list:
  - listitem:
    - button "proj workspace status working":
      - text: proj
      - img "workspace status working"
  - listitem:
    - button "other workspace status unknown":
      - text: other
      - img "workspace status unknown"
- list:
  - listitem: Agents
  - listitem:
    - 'button "proj-dir: opencode agent status working"':
      - text: "proj-dir: opencode"
      - img "agent status working"
  - listitem:
    - 'button "other-dir: opencode agent status unknown"':
      - text: "other-dir: opencode"
      - img "agent status unknown"
- button "Toggle Sidebar"
- main:
  - button "Toggle Sidebar"
  - navigation "Project sections":
    - link "Files":
      - /url: /projects/proj/dashboard
    - link "Board":
      - /url: /projects/proj/board
    - link "Graph":
      - /url: /projects/proj/graph
    - link "Git":
      - /url: /projects/proj/git
    - link "Herdr":
      - /url: /projects/proj/herdr
  - button "Open terminal"
  - heading "Files" [level=1]
  - button "New file"
  - button "New task"
  - button "Hide hidden files" [pressed]
  - searchbox "Search"
  - list:
    - listitem:
      - button "Expand knowledge"
      - button "knowledge"
    - listitem:
      - button "Expand tasks"
      - button "tasks"
    - listitem:
      - button "README.md"
    - listitem:
      - button "tasks.md"
  - heading "Favorites" [level=2]
  - paragraph: No favorites yet.
  - heading "Recent" [level=2]
  - paragraph: No recent files.
  - button "Collapse agent for dashboard" [expanded]: dashboard off
  - text: Agent kind
  - combobox "Agent kind":
    - option "opencode" [selected]
    - option "codex"
  - button "Start agent for dashboard": Start agent
  - paragraph: herdr will open a tab named “dashboard” and run a opencode agent on it.
  - button "Toggle file explorer"
  - button "Toggle details"
  - button "Add to favorites"
  - button "Refresh explorer and file"
  - button "File actions"
  - button "Edit"
  - text: "not found: projects/proj/dashboard projects/proj/dashboard"
  - code
  - complementary: Details On this page Content Tags No tags
```

# Test source

```ts
  1   | import { test, expect, type Locator } from '@playwright/test'
  2   | import fs from 'node:fs'
  3   | import os from 'node:os'
  4   | import path from 'node:path'
  5   | 
  6   | const treeButton = (explorer: Locator, name: string) =>
  7   |   explorer.locator('.knowledge-tree').getByRole('button', { name, exact: true })
  8   | 
  9   | async function fillMonacoEditor(
  10  |   page: import('@playwright/test').Page,
  11  |   content: string,
  12  |   label = 'File editor',
  13  | ) {
  14  |   // Monaco's input textarea is sized to 0xN (kept hidden to assistive tech),
  15  |   // so click the visible editor container that hosts the labeled input.
  16  |   const editor = page.locator('.monaco-editor', { has: page.getByLabel(label) })
  17  |   await editor.click()
  18  |   await page.keyboard.press('ControlOrMeta+A')
  19  |   await page.keyboard.insertText(content)
  20  | }
  21  | 
  22  | // The web UI uses a custom modal (AppDialogs) instead of native browser
  23  | // prompt/confirm/alert dialogs. This helper accepts it: it fills a value when
  24  | // one is given (prompt) and otherwise just confirms (confirm/alert).
  25  | async function acceptAppDialog(
  26  |   page: import('@playwright/test').Page,
  27  |   value?: string,
  28  | ) {
  29  |   const dialog = page.getByTestId('app-dialog')
  30  |   await expect(dialog).toBeVisible()
  31  |   if (value !== undefined) {
  32  |     await dialog.getByTestId('app-dialog-input').fill(value)
  33  |   }
  34  |   await dialog.getByTestId('app-dialog-ok').click()
  35  | }
  36  | 
  37  | test.beforeEach(async ({ page }) => {
  38  |   await page.goto('/projects/proj/dashboard')
  39  | })
  40  | 
  41  | test('dashboard shows project file explorer with README by default', async ({ page }) => {
  42  |   await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  43  |   await expect(
  44  |     page.getByText('This project tracks tasks and knowledge.', { exact: true }),
> 45  |   ).toBeVisible()
      |     ^ Error: expect(locator).toBeVisible() failed
  46  |   const explorer = page.locator('.knowledge-explorer')
  47  |   await expect(explorer).toContainText('README.md')
  48  |   await expect(explorer).toContainText('knowledge')
  49  |   await expect(explorer).toContainText('tasks')
  50  | })
  51  | 
  52  | test('dashboard shows a task status badge on the containing directory', async ({ page }) => {
  53  |   const explorer = page.locator('.knowledge-explorer')
  54  |   await explorer.getByRole('button', { name: 'Expand tasks' }).click()
  55  |   const dirRow = explorer.locator('.knowledge-tree-row', { hasText: 'e2e-status-change-target' })
  56  |   await expect(dirRow.locator('.badge.status-doing')).toBeVisible()
  57  |   await expect(dirRow.locator('.badge.status-doing')).toHaveText('doing')
  58  |   await explorer.getByRole('button', { name: 'Expand e2e-status-change-target' }).click()
  59  |   const taskRow = explorer.locator('.knowledge-tree-row', { hasText: 'task.md' })
  60  |   await expect(taskRow.locator('.badge')).toHaveCount(0)
  61  | })
  62  | 
  63  | test('dashboard opens a markdown file from the explorer', async ({ page }) => {
  64  |   const explorer = page.locator('.knowledge-explorer')
  65  |   await expect(explorer).toContainText('tasks.md')
  66  |   await treeButton(explorer, 'tasks.md').click()
  67  |   await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  68  |   await expect(
  69  |     treeButton(explorer, 'tasks.md'),
  70  |   ).toHaveClass(/active/)
  71  | })
  72  | 
  73  | test('dashboard opens a directory README from the explorer', async ({ page }) => {
  74  |   const explorer = page.locator('.knowledge-explorer')
  75  |   await treeButton(explorer, 'knowledge').click()
  76  |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge$/)
  77  |   await expect(page.getByText('All knowledge lives here.')).toBeVisible()
  78  |   await expect(page.locator('.markdown-body a', { hasText: 'docs' })).toHaveAttribute(
  79  |     'href',
  80  |     '/projects/proj/dashboard/files/knowledge/docs',
  81  |   )
  82  |   await expect(treeButton(explorer, 'knowledge')).toHaveClass(/active/)
  83  |   await expect(page.getByRole('button', { name: 'Edit' })).toHaveCount(0)
  84  |   await expect(page.getByRole('button', { name: 'Rename' })).toHaveCount(0)
  85  |   await page.getByRole('button', { name: 'File actions' }).click()
  86  |   await expect(page.getByRole('menuitem', { name: 'Move' })).toBeVisible()
  87  |   await expect(page.getByRole('menuitem', { name: 'Duplicate' })).toBeVisible()
  88  |   await expect(page.getByRole('menuitem', { name: 'Delete' })).toBeVisible()
  89  | })
  90  | 
  91  | test('dashboard lists a directory that has no README', async ({ page }) => {
  92  |   const explorer = page.locator('.knowledge-explorer')
  93  |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  94  |   await treeButton(explorer, 'docs').click()
  95  |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/docs$/)
  96  |   await expect(page.getByRole('heading', { name: 'Directories', level: 2 })).toBeVisible()
  97  |   await expect(page.getByRole('heading', { name: 'Files', level: 2 })).toBeVisible()
  98  |   await expect(page.locator('.markdown-body a', { hasText: 'recipes/' })).toHaveAttribute(
  99  |     'href',
  100 |     '/projects/proj/dashboard/files/knowledge/docs/recipes',
  101 |   )
  102 |   await expect(page.locator('.markdown-body a', { hasText: 'guide.md' })).toHaveAttribute(
  103 |     'href',
  104 |     '/projects/proj/dashboard/files/knowledge/docs/guide.md',
  105 |   )
  106 | })
  107 | 
  108 | test('a directory link in a README opens the subdirectory listing', async ({ page }) => {
  109 |   const explorer = page.locator('.knowledge-explorer')
  110 |   await treeButton(explorer, 'knowledge').click()
  111 |   await expect(page.getByText('All knowledge lives here.')).toBeVisible()
  112 |   await page.locator('.markdown-body a', { hasText: 'docs' }).click()
  113 |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/docs$/)
  114 |   await expect(page.getByRole('heading', { name: 'Files', level: 2 })).toBeVisible()
  115 |   await expect(page.locator('.markdown-body a', { hasText: 'guide.md' })).toBeVisible()
  116 | })
  117 | 
  118 | test('dashboard keeps the open file in the URL across a reload', async ({ page }) => {
  119 |   const explorer = page.locator('.knowledge-explorer')
  120 |   await treeButton(explorer, 'tasks.md').click()
  121 |   await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  122 |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/tasks\.md$/)
  123 |   await page.reload()
  124 |   await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  125 |   await expect(
  126 |     treeButton(explorer, 'tasks.md'),
  127 |   ).toHaveClass(/active/)
  128 | })
  129 | 
  130 | test('dashboard shows a content outline for markdown files', async ({ page }) => {
  131 |   const explorer = page.locator('.knowledge-explorer')
  132 |   await treeButton(explorer, 'tasks.md').click()
  133 |   await expect(page.getByText('Task tracking lives here.')).toBeVisible()
  134 |   const outline = page.locator('.outline')
  135 |   await expect(outline).toContainText('Content')
  136 |   await expect(outline.getByRole('link', { name: 'Tasks' })).toBeVisible()
  137 |   await expect(outline.getByText('Graph')).toHaveCount(0)
  138 | })
  139 | 
  140 | test('the details pane stays visible while the file content scrolls', async ({ page }) => {
  141 |   const explorer = page.locator('.knowledge-explorer')
  142 |   await treeButton(explorer, 'tasks.md').click()
  143 |   const outlineHeader = page.locator('.outline-header')
  144 |   await expect(outlineHeader).toBeVisible()
  145 |   const scroller = page.locator('.knowledge-files')
```