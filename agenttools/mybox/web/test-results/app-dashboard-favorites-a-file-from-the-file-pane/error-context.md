# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: app.spec.ts >> dashboard favorites a file from the file pane
- Location: tests/e2e/app.spec.ts:323:1

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.click: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByRole('button', { name: '☆ Favorite' })

```

# Page snapshot

```yaml
- generic [ref=e3]:
  - generic [ref=e6]:
    - list [ref=e8]:
      - listitem [ref=e9]:
        - button "Go to top" [ref=e10] [cursor=pointer]:
          - generic [ref=e15]: mybox
    - generic [ref=e16]:
      - list [ref=e17]:
        - listitem [ref=e18]:
          - link "Workspaces" [ref=e19] [cursor=pointer]:
            - /url: /projects
        - listitem [ref=e31]:
          - link "Board" [ref=e32] [cursor=pointer]:
            - /url: /board
        - listitem [ref=e36]:
          - link "Stats" [ref=e37] [cursor=pointer]:
            - /url: /stats
      - list [ref=e41]:
        - listitem [ref=e42]:
          - button "proj workspace status working" [ref=e43] [cursor=pointer]:
            - generic [ref=e47]: proj
            - img "workspace status working" [ref=e49]
        - listitem [ref=e50]:
          - button "other workspace status unknown" [ref=e51] [cursor=pointer]:
            - generic [ref=e55]: other
            - img "workspace status unknown" [ref=e57]
      - list [ref=e58]:
        - listitem [ref=e59]: Agents
        - listitem [ref=e63]:
          - 'button "proj-dir: opencode agent status working" [ref=e64] [cursor=pointer]':
            - generic [ref=e68]:
              - generic [ref=e69]: "proj-dir: opencode"
              - img "agent status working" [ref=e70]
        - listitem [ref=e71]:
          - 'button "other-dir: opencode agent status unknown" [ref=e72] [cursor=pointer]':
            - generic [ref=e76]:
              - generic [ref=e77]: "other-dir: opencode"
              - img "agent status unknown" [ref=e78]
    - button "Toggle Sidebar" [ref=e79]
  - main [ref=e80]:
    - generic [ref=e82]:
      - button "Toggle Sidebar" [ref=e83]
      - navigation "Project sections" [ref=e85]:
        - link "Files" [ref=e86] [cursor=pointer]:
          - /url: /projects/proj/dashboard/files/tasks.md
        - link "Board" [ref=e89] [cursor=pointer]:
          - /url: /projects/proj/board
        - link "Graph" [ref=e92] [cursor=pointer]:
          - /url: /projects/proj/graph
        - link "Git" [ref=e98] [cursor=pointer]:
          - /url: /projects/proj/git
        - link "Herdr" [ref=e103] [cursor=pointer]:
          - /url: /projects/proj/herdr
      - button "Open terminal" [ref=e107] [cursor=pointer]
    - generic [ref=e112]:
      - generic [ref=e116]:
        - generic [ref=e117]:
          - heading "Files" [level=1] [ref=e118]
          - generic [ref=e119]:
            - button "New file" [ref=e120] [cursor=pointer]:
              - generic [ref=e121]: File
            - button "New task" [ref=e122] [cursor=pointer]:
              - generic [ref=e123]: Task
        - searchbox "Search" [ref=e126]
        - list [ref=e127]:
          - listitem [ref=e128]:
            - button "Expand knowledge" [ref=e129] [cursor=pointer]
            - button "knowledge" [ref=e132] [cursor=pointer]
          - listitem [ref=e133]:
            - button "Expand tasks" [ref=e134] [cursor=pointer]
            - button "tasks" [ref=e137] [cursor=pointer]
          - listitem [ref=e138]:
            - button "README.md" [ref=e139] [cursor=pointer]
          - listitem [ref=e140]:
            - button "tasks.md" [active] [ref=e141] [cursor=pointer]
            - generic [ref=e142]: done
        - generic [ref=e143]:
          - generic [ref=e144]:
            - heading "Favorites" [level=2] [ref=e145]
            - paragraph [ref=e146]: No favorites yet.
          - generic [ref=e147]:
            - heading "Recent" [level=2] [ref=e148]
            - list [ref=e149]:
              - listitem [ref=e150]:
                - button "tasks.md" [ref=e151] [cursor=pointer]
              - listitem [ref=e157]:
                - button "README.md" [ref=e158] [cursor=pointer]
      - generic [ref=e164]:
        - generic [ref=e165]:
          - generic [ref=e166]:
            - button "tasks.md" [ref=e167] [cursor=pointer]
            - button "Close tasks.md" [ref=e169] [cursor=pointer]
          - generic [ref=e173]:
            - button "README.md" [ref=e174] [cursor=pointer]
            - button "Close README.md" [ref=e176] [cursor=pointer]
        - generic [ref=e181]:
          - generic [ref=e182]:
            - button "Collapse agent for tasks.md" [expanded] [ref=e183] [cursor=pointer]:
              - generic "tasks.md" [ref=e189]
              - generic [ref=e190]: "off"
            - generic [ref=e191]:
              - generic [ref=e192]:
                - generic [ref=e193]: Agent kind
                - combobox "Agent kind" [ref=e194] [cursor=pointer]:
                  - option "opencode" [selected]
                  - option "codex"
              - button "Start agent for tasks.md" [ref=e195] [cursor=pointer]: Start agent
              - paragraph [ref=e196]: herdr will open a tab named “tasks.md” and run a opencode agent on it.
          - generic [ref=e197]:
            - generic [ref=e198]:
              - generic [ref=e200]:
                - button "Toggle file explorer" [ref=e201]
                - button "Toggle details" [ref=e202]
                - button "Add to favorites" [ref=e203]
                - button "File actions" [ref=e205]
                - button "Edit" [ref=e206]
              - generic [ref=e208]:
                - generic [ref=e209]:
                  - generic [ref=e210]: tasks.md
                  - generic [ref=e211]: docs
                  - generic [ref=e212]: meta
                - generic [ref=e213]:
                  - heading "Metadata" [level=3] [ref=e214]
                  - generic [ref=e215]:
                    - generic [ref=e216]:
                      - term [ref=e217]: status
                      - definition [ref=e218]:
                        - generic [ref=e219]: done
                    - generic [ref=e220]:
                      - term [ref=e221]: priority
                      - definition [ref=e222]:
                        - generic [ref=e223]: high
                    - generic [ref=e224]:
                      - term [ref=e225]: tags
                      - definition [ref=e226]:
                        - generic [ref=e227]: docs
                        - generic [ref=e228]: meta
                - generic [ref=e230]:
                  - heading "Tasks" [level=1] [ref=e231]
                  - paragraph [ref=e232]: Metadata added.
            - complementary [ref=e234]:
              - generic [ref=e239]:
                - generic [ref=e240]: Details
                - generic [ref=e241]: On this page
              - generic [ref=e242]:
                - generic [ref=e243]:
                  - generic [ref=e244]: Content
                  - link "Tasks" [ref=e248] [cursor=pointer]:
                    - /url: "#tasks"
                - generic [ref=e249]:
                  - generic [ref=e250]: Tags
                  - generic [ref=e254]:
                    - generic [ref=e255]: docs
                    - generic [ref=e256]: meta
```

# Test source

```ts
  226 | })
  227 | 
  228 | test('dragging the resize handle grows the internal terminal', async ({ page }) => {
  229 |   await page.getByRole('button', { name: 'Open terminal' }).click()
  230 |   const panel = page.locator('.terminal-panel')
  231 |   await expect(panel).toBeVisible()
  232 | 
  233 |   const xterm = page.locator('.terminal-xterm')
  234 |   await expect(xterm).toBeVisible()
  235 |   const heightBefore = await xterm.evaluate((el) => el.clientHeight)
  236 |   expect(heightBefore).toBeGreaterThan(100)
  237 | 
  238 |   const handle = page.locator('[data-testid="terminal-resize-handle"]')
  239 |   const box = await handle.boundingBox()
  240 |   expect(box).not.toBeNull()
  241 |   await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2)
  242 |   await page.mouse.down()
  243 |   await page.mouse.move(box!.x + 20, box!.y - 140, { steps: 8 })
  244 |   await page.mouse.up()
  245 | 
  246 |   await expect
  247 |     .poll(async () => page.locator('.terminal-xterm').evaluate((el) => el.clientHeight))
  248 |     .toBeGreaterThan(heightBefore + 100)
  249 | })
  250 | 
  251 | test('terminal session persists across a browser reload', async ({ page }) => {
  252 |   await page.getByRole('button', { name: 'Open terminal' }).click()
  253 |   const panel = page.locator('.terminal-panel')
  254 |   await expect(panel).toBeVisible()
  255 | 
  256 |   // Focus the terminal and run a command whose echoed output can be verified
  257 |   // as replayed history after reconnecting to the same session on reload.
  258 |   await page.locator('.terminal-xterm').click()
  259 |   await page.keyboard.type('echo PERSIST_RELOAD_99')
  260 |   await page.keyboard.press('Enter')
  261 |   await expect(page.locator('.terminal-xterm')).toContainText('PERSIST_RELOAD_99')
  262 | 
  263 |   const sessionIdBefore = await page.evaluate(() => {
  264 |     const m = JSON.parse(window.localStorage.getItem('mybox_terminals_v1') || '{}')
  265 |     const first = Object.values(m)[0] as { tabs?: { sessionId?: string }[] } | undefined
  266 |     return first?.tabs?.[0]?.sessionId ?? null
  267 |   })
  268 |   expect(sessionIdBefore).toBeTruthy()
  269 | 
  270 |   await page.reload()
  271 |   await expect(page.locator('.terminal-panel')).toBeVisible()
  272 | 
  273 |   const sessionIdAfter = await page.evaluate(() => {
  274 |     const m = JSON.parse(window.localStorage.getItem('mybox_terminals_v1') || '{}')
  275 |     const first = Object.values(m)[0] as { tabs?: { sessionId?: string }[] } | undefined
  276 |     return first?.tabs?.[0]?.sessionId ?? null
  277 |   })
  278 |   expect(sessionIdAfter).toBe(sessionIdBefore)
  279 | 
  280 |   // Reattached to the same server-side session: historical output is replayed.
  281 |   await expect(page.locator('.terminal-xterm')).toContainText('PERSIST_RELOAD_99')
  282 | })
  283 | 
  284 | test('dashboard edits and saves a file', async ({ page }) => {
  285 |   const explorer = page.locator('.knowledge-explorer')
  286 |   await treeButton(explorer, 'tasks.md').click()
  287 |   await page.getByRole('button', { name: 'Edit' }).click()
  288 |   await fillMonacoEditor(page, '# Task tracking\n\nEdited content.\n')
  289 |   await page.getByRole('button', { name: 'Save' }).click()
  290 |   await expect(page.getByText('Saved.')).toBeVisible()
  291 |   await expect(page.getByText('Edited content.')).toBeVisible()
  292 | })
  293 | 
  294 | test('dashboard edits frontmatter metadata separately from the body', async ({ page }) => {
  295 |   const explorer = page.locator('.knowledge-explorer')
  296 |   await treeButton(explorer, 'tasks.md').click()
  297 |   await page.getByRole('button', { name: 'Edit' }).click()
  298 |   // the metadata form is collapsed by default; expand it first
  299 |   await page.getByRole('button', { name: /Metadata/ }).click()
  300 |   await page.getByLabel('Metadata status').selectOption('done')
  301 |   await page.getByLabel('Metadata priority').selectOption('high')
  302 |   await page.getByLabel('Metadata tags').fill('docs, meta')
  303 |   await fillMonacoEditor(page, '# Tasks\n\nMetadata added.\n')
  304 |   await page.getByRole('button', { name: 'Save' }).click()
  305 | 
  306 |   await expect(page.locator('.frontmatter-card .badge.status-done')).toBeVisible()
  307 |   await expect(page.locator('.frontmatter-card .badge.priority-high')).toBeVisible()
  308 |   await expect(page.locator('.frontmatter-card .badge.tag').filter({ hasText: 'docs' })).toBeVisible()
  309 |   await expect(page.getByText('Metadata added.')).toBeVisible()
  310 | 
  311 |   const res = await page.request.get('/api/files/content?path=tasks.md')
  312 |   const body = (await res.json()) as { content: string }
  313 |   expect(body.content).toContain('status: done')
  314 |   expect(body.content).toContain('priority: high')
  315 |   expect(body.content).toContain('tags')
  316 |   expect(body.content).toContain('docs')
  317 |   expect(body.content).toContain('Metadata added.')
  318 | 
  319 |   const row = explorer.locator('.knowledge-tree-row', { hasText: 'tasks.md' })
  320 |   await expect(row.locator('.badge.status-done')).toBeVisible()
  321 | })
  322 | 
  323 | test('dashboard favorites a file from the file pane', async ({ page }) => {
  324 |   const explorer = page.locator('.knowledge-explorer')
  325 |   await treeButton(explorer, 'tasks.md').click()
> 326 |   await page.getByRole('button', { name: '☆ Favorite' }).click()
      |                                                          ^ Error: locator.click: Test timeout of 30000ms exceeded.
  327 |   await expect(page.getByRole('button', { name: '★ Favorite' })).toBeVisible()
  328 |   const favorites = explorer.locator('.explorer-section').filter({ hasText: 'Favorites' })
  329 |   await expect(favorites).toContainText('tasks.md')
  330 | })
  331 | 
  332 | test('dashboard records recently opened files', async ({ page }) => {
  333 |   const explorer = page.locator('.knowledge-explorer')
  334 |   await treeButton(explorer, 'tasks.md').click()
  335 |   const recent = explorer.locator('.explorer-section').filter({ hasText: 'Recent' })
  336 |   await expect(recent).toContainText('tasks.md')
  337 | })
  338 | 
  339 | test('dashboard moves a file by dragging onto a directory', async ({ page }) => {
  340 |   const explorer = page.locator('.knowledge-explorer')
  341 |   await expect(explorer).toContainText('tasks.md')
  342 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  343 |   await treeButton(explorer, 'tasks.md')
  344 |     .dragTo(explorer.getByRole('button', { name: 'Collapse knowledge' }))
  345 |   await explorer.locator('.search-bar input').fill('knowledge/tasks')
  346 |   const list = explorer.locator('.file-list')
  347 |   await expect(list.getByRole('button', { name: 'knowledge/tasks.md' })).toBeVisible()
  348 | })
  349 | 
  350 | test('dashboard renames a file via Move', async ({ page }) => {
  351 |   page.on('dialog', (d) => d.accept('knowledge/tasks2.md'))
  352 |   const explorer = page.locator('.knowledge-explorer')
  353 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  354 |   await treeButton(explorer, 'tasks.md').click()
  355 |   await page.getByRole('button', { name: 'File actions' }).click()
  356 |   await page.getByRole('menuitem', { name: 'Move' }).click({ force: true })
  357 |   await expect(page.getByText('knowledge/tasks2.md', { exact: true })).toBeVisible()
  358 |   await expect(explorer).toContainText('tasks2.md')
  359 | })
  360 | 
  361 | test('dashboard duplicates a file', async ({ page }) => {
  362 |   page.on('dialog', (d) => d.accept('knowledge/tasks2-copy.md'))
  363 |   const explorer = page.locator('.knowledge-explorer')
  364 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  365 |   await treeButton(explorer, 'tasks2.md').click()
  366 |   await page.getByRole('button', { name: 'File actions' }).click()
  367 |   await page.getByRole('menuitem', { name: 'Duplicate' }).click({ force: true })
  368 |   // Scope to the tree: the opened viewer's meta line shows the same path.
  369 |   await expect(treeButton(explorer, 'tasks2-copy.md')).toBeVisible()
  370 | })
  371 | 
  372 | test('dashboard deletes a file', async ({ page }) => {
  373 |   page.on('dialog', (d) => d.accept())
  374 |   const explorer = page.locator('.knowledge-explorer')
  375 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  376 |   await treeButton(explorer, 'tasks2-copy.md').click()
  377 |   await page.getByRole('button', { name: 'File actions' }).click()
  378 |   await page.getByRole('menuitem', { name: 'Delete' }).click({ force: true })
  379 |   await expect(
  380 |     page.getByText('Select a file from the explorer to view it here.', { exact: true }),
  381 |   ).toBeVisible()
  382 |   await expect(explorer).not.toContainText('tasks2-copy.md')
  383 | })
  384 | 
  385 | test('clicking the mybox brand returns to the unselected projects page', async ({ page }) => {
  386 |   await page.locator('.sidebar-nav').getByRole('link', { name: 'Board', exact: true }).click()
  387 |   await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  388 |   await page.getByRole('button', { name: 'Go to top' }).click()
  389 |   await expect(page.getByRole('heading', { name: 'Workspaces', level: 1 })).toBeVisible()
  390 |   await expect(page.locator('.sidebar-projects')).toContainText('proj')
  391 |   await expect(page.locator('.sidebar-nav').getByRole('link', { name: 'Dashboard' })).toHaveCount(0)
  392 | })
  393 | 
  394 | test('project tabs switch between files, board and graph', async ({ page }) => {
  395 |   const tabs = page.locator('.project-tabs')
  396 |   await expect(tabs.getByRole('link', { name: 'Files' })).toBeVisible()
  397 |   await tabs.getByRole('link', { name: 'Board' }).click()
  398 |   await expect(page).toHaveURL(/\/projects\/proj\/board$/)
  399 |   await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  400 |   await expect(tabs.getByRole('link', { name: 'Board' })).toHaveClass(/bg-accent/)
  401 |   await tabs.getByRole('link', { name: 'Graph' }).click()
  402 |   await expect(page).toHaveURL(/\/projects\/proj\/graph$/)
  403 |   await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
  404 |   await tabs.getByRole('link', { name: 'Git' }).click()
  405 |   await expect(page).toHaveURL(/\/projects\/proj\/git$/)
  406 |   await expect(page.getByRole('heading', { name: 'No git repository' })).toBeVisible()
  407 |   await tabs.getByRole('link', { name: 'Files' }).click()
  408 |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard/)
  409 |   await expect(tabs.getByRole('link', { name: 'Files' })).toHaveClass(/bg-accent/)
  410 | })
  411 | 
  412 | test('sidebar lists projects and switches between them', async ({ page }) => {
  413 |   const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-side-'))
  414 |   const res = await page.request.post('/api/projects', { data: { path: dir } })
  415 |   expect(res.ok()).toBeTruthy()
  416 |   await page.reload()
  417 |   const list = page.locator('.sidebar-projects')
  418 |   await expect(list).toContainText('proj')
  419 |   const other = path.basename(dir)
  420 |   await list.getByRole('button', { name: other }).click()
  421 |   await expect(page).toHaveURL(new RegExp(`/projects/${other}/dashboard$`))
  422 | })
  423 | 
  424 | test('sidebar board shows tasks across projects', async ({ page }) => {
  425 |   const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-cross-'))
  426 |   const added = await page.request.post('/api/projects', { data: { path: dir } })
```