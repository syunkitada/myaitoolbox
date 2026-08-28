# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: app.spec.ts >> search finds knowledge and opens it in the dashboard
- Location: tests/e2e/app.spec.ts:407:1

# Error details

```
Test timeout of 30000ms exceeded.
```

```
Error: locator.click: Test timeout of 30000ms exceeded.
Call log:
  - waiting for getByRole('button', { name: 'Search', exact: true })

```

# Page snapshot

```yaml
- generic [ref=e3]:
  - generic [ref=e6]:
    - generic [ref=e7]:
      - list [ref=e8]:
        - listitem [ref=e9]:
          - button "Go to top" [ref=e10] [cursor=pointer]:
            - generic [ref=e15]: mybox
      - list [ref=e16]:
        - listitem [ref=e17]:
          - link "Workspaces" [ref=e18] [cursor=pointer]:
            - /url: /projects
        - listitem [ref=e30]:
          - link "Board" [ref=e31] [cursor=pointer]:
            - /url: /board
      - list [ref=e35]:
        - listitem [ref=e36]:
          - button "proj workspace status working" [ref=e37] [cursor=pointer]:
            - generic [ref=e41]: proj
            - img "workspace status working" [ref=e43]
        - listitem [ref=e44]:
          - button "mybox-e2e-side-x5fLoo" [ref=e45] [cursor=pointer]
      - list [ref=e50]:
        - listitem [ref=e51]: Agents
        - listitem [ref=e55]:
          - button "proj · proj-dir agent status working OC | stub agent" [ref=e56] [cursor=pointer]:
            - generic [ref=e60]:
              - generic [ref=e61]:
                - generic [ref=e62]: proj
                - generic "/home/stub/proj-dir" [ref=e63]: · proj-dir
                - img "agent status working" [ref=e64]
              - generic "OC | stub agent" [ref=e65]
    - button "Toggle Sidebar" [ref=e66]
  - main [ref=e67]:
    - generic [ref=e69]:
      - button "Toggle Sidebar" [ref=e70]
      - navigation "Project sections" [ref=e72]:
        - link "Files" [ref=e73] [cursor=pointer]:
          - /url: /projects/proj/dashboard
        - link "Board" [ref=e74] [cursor=pointer]:
          - /url: /projects/proj/board
        - link "Graph" [ref=e75] [cursor=pointer]:
          - /url: /projects/proj/graph
        - link "Herdr" [ref=e76] [cursor=pointer]:
          - /url: /projects/proj/herdr
      - button "Open terminal" [ref=e77] [cursor=pointer]
    - generic [ref=e82]:
      - generic [ref=e86]:
        - generic [ref=e87]:
          - heading "Files" [level=1] [ref=e88]
          - generic [ref=e89]:
            - button "New file" [ref=e90] [cursor=pointer]:
              - generic [ref=e91]: File
            - button "New task" [ref=e92] [cursor=pointer]:
              - generic [ref=e93]: Task
        - searchbox "Search" [ref=e96]
        - list [ref=e97]:
          - listitem [ref=e98]:
            - button "Expand knowledge" [ref=e99] [cursor=pointer]
            - button "knowledge" [ref=e102] [cursor=pointer]
          - listitem [ref=e103]:
            - button "Expand tasks" [ref=e104] [cursor=pointer]
            - button "tasks" [ref=e107] [cursor=pointer]
          - listitem [ref=e108]:
            - button "README.md" [ref=e109] [cursor=pointer]
        - generic [ref=e110]:
          - generic [ref=e111]:
            - heading "Favorites" [level=2] [ref=e112]
            - paragraph [ref=e113]: No favorites yet.
          - generic [ref=e114]:
            - heading "Recent" [level=2] [ref=e115]
            - list [ref=e116]:
              - listitem [ref=e117]:
                - button "README.md" [ref=e118] [cursor=pointer]
              - listitem [ref=e124]:
                - button "knowledge/tasks2.md" [ref=e125] [cursor=pointer]
              - listitem [ref=e131]:
                - button "knowledge/docs" [ref=e132] [cursor=pointer]
      - generic [ref=e141]:
        - generic [ref=e142]:
          - generic [ref=e144]:
            - button "Toggle file explorer" [ref=e145]
            - button "Toggle details" [ref=e146]
            - button "☆ Favorite" [ref=e147]
            - button "File actions" [ref=e149]
            - button "Edit" [ref=e150]
          - generic [ref=e152]:
            - generic [ref=e153]: README.md
            - generic [ref=e156]:
              - heading "Mybox" [level=1] [ref=e157]
              - paragraph [ref=e158]: This project tracks tasks and knowledge.
        - complementary [ref=e160]:
          - generic [ref=e165]:
            - generic [ref=e166]: Details
            - generic [ref=e167]: On this page
          - generic [ref=e168]:
            - generic [ref=e169]:
              - generic [ref=e170]: Content
              - link "Mybox" [ref=e174] [cursor=pointer]:
                - /url: "#mybox"
            - generic [ref=e175]:
              - generic [ref=e176]: Tags
              - generic [ref=e180]: No tags
```

# Test source

```ts
  308 | 
  309 | test('dashboard favorites a file from the file pane', async ({ page }) => {
  310 |   const explorer = page.locator('.knowledge-explorer')
  311 |   await treeButton(explorer, 'tasks.md').click()
  312 |   await page.getByRole('button', { name: '☆ Favorite' }).click()
  313 |   await expect(page.getByRole('button', { name: '★ Favorite' })).toBeVisible()
  314 |   const favorites = explorer.locator('.explorer-section').filter({ hasText: 'Favorites' })
  315 |   await expect(favorites).toContainText('tasks.md')
  316 | })
  317 | 
  318 | test('dashboard records recently opened files', async ({ page }) => {
  319 |   const explorer = page.locator('.knowledge-explorer')
  320 |   await treeButton(explorer, 'tasks.md').click()
  321 |   const recent = explorer.locator('.explorer-section').filter({ hasText: 'Recent' })
  322 |   await expect(recent).toContainText('tasks.md')
  323 | })
  324 | 
  325 | test('dashboard moves a file by dragging onto a directory', async ({ page }) => {
  326 |   const explorer = page.locator('.knowledge-explorer')
  327 |   await expect(explorer).toContainText('tasks.md')
  328 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  329 |   await treeButton(explorer, 'tasks.md')
  330 |     .dragTo(explorer.getByRole('button', { name: 'Collapse knowledge' }))
  331 |   await explorer.locator('.search-bar input').fill('knowledge/tasks')
  332 |   const list = explorer.locator('.file-list')
  333 |   await expect(list.getByRole('button', { name: 'knowledge/tasks.md' })).toBeVisible()
  334 | })
  335 | 
  336 | test('dashboard renames a file via Move', async ({ page }) => {
  337 |   page.on('dialog', (d) => d.accept('knowledge/tasks2.md'))
  338 |   const explorer = page.locator('.knowledge-explorer')
  339 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  340 |   await treeButton(explorer, 'tasks.md').click()
  341 |   await page.getByRole('button', { name: 'File actions' }).click()
  342 |   await page.getByRole('menuitem', { name: 'Move' }).click({ force: true })
  343 |   await expect(page.getByText('knowledge/tasks2.md', { exact: true })).toBeVisible()
  344 |   await expect(explorer).toContainText('tasks2.md')
  345 | })
  346 | 
  347 | test('dashboard duplicates a file', async ({ page }) => {
  348 |   page.on('dialog', (d) => d.accept('knowledge/tasks2-copy.md'))
  349 |   const explorer = page.locator('.knowledge-explorer')
  350 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  351 |   await treeButton(explorer, 'tasks2.md').click()
  352 |   await page.getByRole('button', { name: 'File actions' }).click()
  353 |   await page.getByRole('menuitem', { name: 'Duplicate' }).click({ force: true })
  354 |   // Scope to the tree: the opened viewer's meta line shows the same path.
  355 |   await expect(treeButton(explorer, 'tasks2-copy.md')).toBeVisible()
  356 | })
  357 | 
  358 | test('dashboard deletes a file', async ({ page }) => {
  359 |   page.on('dialog', (d) => d.accept())
  360 |   const explorer = page.locator('.knowledge-explorer')
  361 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  362 |   await treeButton(explorer, 'tasks2-copy.md').click()
  363 |   await page.getByRole('button', { name: 'File actions' }).click()
  364 |   await page.getByRole('menuitem', { name: 'Delete' }).click({ force: true })
  365 |   await expect(
  366 |     page.getByText('Select a file from the explorer to view it here.', { exact: true }),
  367 |   ).toBeVisible()
  368 |   await expect(explorer).not.toContainText('tasks2-copy.md')
  369 | })
  370 | 
  371 | test('clicking the mybox brand returns to the unselected projects page', async ({ page }) => {
  372 |   await page.locator('.sidebar-nav').getByRole('link', { name: 'Board', exact: true }).click()
  373 |   await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  374 |   await page.getByRole('button', { name: 'Go to top' }).click()
  375 |   await expect(page.getByRole('heading', { name: 'Workspaces', level: 1 })).toBeVisible()
  376 |   await expect(page.locator('.sidebar-projects')).toContainText('proj')
  377 |   await expect(page.locator('.sidebar-nav').getByRole('link', { name: 'Dashboard' })).toHaveCount(0)
  378 | })
  379 | 
  380 | test('project tabs switch between files, board and graph', async ({ page }) => {
  381 |   const tabs = page.locator('.project-tabs')
  382 |   await expect(tabs.getByRole('link', { name: 'Files' })).toBeVisible()
  383 |   await tabs.getByRole('link', { name: 'Board' }).click()
  384 |   await expect(page).toHaveURL(/\/projects\/proj\/board$/)
  385 |   await expect(page.getByRole('heading', { name: 'Board' })).toBeVisible()
  386 |   await expect(tabs.getByRole('link', { name: 'Board' })).toHaveClass(/bg-accent/)
  387 |   await tabs.getByRole('link', { name: 'Graph' }).click()
  388 |   await expect(page).toHaveURL(/\/projects\/proj\/graph$/)
  389 |   await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
  390 |   await tabs.getByRole('link', { name: 'Files' }).click()
  391 |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard/)
  392 |   await expect(tabs.getByRole('link', { name: 'Files' })).toHaveClass(/bg-accent/)
  393 | })
  394 | 
  395 | test('sidebar lists projects and switches between them', async ({ page }) => {
  396 |   const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-side-'))
  397 |   const res = await page.request.post('/api/projects', { data: { path: dir } })
  398 |   expect(res.ok()).toBeTruthy()
  399 |   await page.reload()
  400 |   const list = page.locator('.sidebar-projects')
  401 |   await expect(list).toContainText('proj')
  402 |   const other = path.basename(dir)
  403 |   await list.getByRole('button', { name: other }).click()
  404 |   await expect(page).toHaveURL(new RegExp(`/projects/${other}/dashboard$`))
  405 | })
  406 | 
  407 | test('search finds knowledge and opens it in the dashboard', async ({ page }) => {
> 408 |   await page.getByRole('button', { name: 'Search', exact: true }).click()
      |                                                                   ^ Error: locator.click: Test timeout of 30000ms exceeded.
  409 |   await expect(page).toHaveURL(/\/projects\/proj\/search$/)
  410 |   await page.getByPlaceholder('Search…').fill('phase6')
  411 |   await page.getByPlaceholder('Search…').press('Enter')
  412 |   await expect(page.getByRole('heading', { name: 'Search' })).toBeVisible()
  413 |   await page.getByRole('button', { name: 'Phase 6' }).click()
  414 |   await expect(page).toHaveURL(/\/projects\/proj\/dashboard\/files\/knowledge\/notes\/phase6\.md$/)
  415 |   await expect(page.getByText('The HTTP API is done.')).toBeVisible()
  416 | })
  417 | 
  418 | test('sidebar board shows tasks across projects', async ({ page }) => {
  419 |   const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mybox-e2e-cross-'))
  420 |   const added = await page.request.post('/api/projects', { data: { path: dir } })
  421 |   expect(added.ok()).toBeTruthy()
  422 |   const other = path.basename(dir)
  423 |   const taskName = `Cross task ${other}`
  424 |   const created = await page.request.post('/api/tasks', {
  425 |     headers: { 'X-Project': other },
  426 |     data: { name: taskName },
  427 |   })
  428 |   expect(created.ok()).toBeTruthy()
  429 | 
  430 |   await page.goto('/projects/proj/dashboard')
  431 |   await page.locator('.project-tabs').getByRole('link', { name: 'Board', exact: true }).click()
  432 |   await expect(page.locator('.board-card').first()).toBeVisible()
  433 |   await page.locator('.sidebar-nav').getByRole('link', { name: 'Board', exact: true }).click()
  434 |   await expect(page).toHaveURL(/\/board$/)
  435 |   await expect(page.getByText('全プロジェクト横断ビュー')).toBeVisible()
  436 |   const cards = page.locator('.board-card')
  437 |   await expect(cards.filter({ hasText: taskName })).toBeVisible()
  438 |   await expect(cards.filter({ hasText: 'Ship the web UI' })).toBeVisible()
  439 | })
  440 | 
  441 | test('favicon mirrors the aggregated agent status and updates dynamically', async ({ page }) => {
  442 |   await page.goto('/projects/proj/herdr')
  443 | 
  444 |   // The stub agent reports "working"; the favicon becomes its blue dot PNG.
  445 |   const favicon = page.locator('link[rel="icon"]')
  446 |   await expect(favicon).toHaveAttribute('href', /data:image\/png/)
  447 |   const workingHref = await favicon.getAttribute('href')
  448 |   expect(workingHref).toBeTruthy()
  449 | 
  450 |   // Once polled data flips the agent to "blocked" the favicon is redrawn.
  451 |   await page.route('**/api/herdr/overview', async (route) => {
  452 |     const res = await route.fetch()
  453 |     const body = { ...(await res.json()) }
  454 |     body.agents = [{ ...body.agents[0], status: 'blocked' }]
  455 |     await route.fulfill({ response: res, json: body })
  456 |   })
  457 |   await expect(favicon).not.toHaveAttribute('href', workingHref!, { timeout: 10000 })
  458 | })
  459 | 
  460 | test('herdr tab shows workspaces and operates agents', async ({ page }) => {
  461 |   await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  462 |   await expect(page.getByRole('heading', { name: 'Herdr', level: 1 })).toBeVisible()
  463 |   await expect(page.getByTestId('herdr-workspace-w7')).toContainText('proj')
  464 |   await expect(page.getByTestId('herdr-workspace-w7')).toContainText('working')
  465 | 
  466 |   await page.locator('.herdr-agent-row').first().click()
  467 |   const detail = page.locator('.herdr-agent-detail')
  468 |   await expect(detail).toBeVisible()
  469 |   await expect(detail.locator('pre')).toContainText('stub output for w7:p1')
  470 | 
  471 |   // the focused agent polls its terminal output every second
  472 |   const pre = detail.locator('pre')
  473 |   const before = await pre.innerText()
  474 |   await expect(pre).not.toHaveText(before, { timeout: 5000 })
  475 | 
  476 |   await page.getByTestId('herdr-prompt-input').fill('run the tests please')
  477 |   await detail.getByRole('button', { name: 'Send' }).click()
  478 |   await expect(detail.locator('.herdr-prompt-notice')).toContainText('prompt submitted')
  479 |   await expect(detail.locator('pre')).toContainText('last prompt: run the tests please')
  480 | })
  481 | 
  482 | test('herdr tab and pane operations work end to end', async ({ page }) => {
  483 |   await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  484 |   const ws = page.getByTestId('herdr-workspace-w7')
  485 |   await expect(ws).toBeVisible()
  486 |   await expect(page.getByTestId('herdr-tab-w7:t1')).toBeVisible()
  487 |   await expect(page.getByTestId('herdr-tab-w7:t2')).toContainText('2:')
  488 |   // the first tab is selected by default; focus is webui-managed
  489 |   await expect(page.getByTestId('herdr-tab-w7:t1')).toHaveAttribute('data-active', 'true')
  490 | 
  491 |   // create a tab via the prompt dialog
  492 |   page.once('dialog', (d) => d.accept('build'))
  493 |   await ws.getByRole('button', { name: '+ New Tab' }).click()
  494 |   const newTab = page.getByTestId('herdr-tab-w7:t3')
  495 |   await expect(newTab).toContainText('build')
  496 | 
  497 |   // selecting a tab stores the focus in the URL instead of calling herdr
  498 |   await newTab.click()
  499 |   await expect(newTab).toHaveAttribute('data-active', 'true')
  500 |   await expect(page.getByTestId('herdr-tab-w7:t1')).toHaveAttribute('data-active', 'false')
  501 |   await expect(page).toHaveURL(/tab=w7(:|%3A)t3/)
  502 | 
  503 |   // clicking a pane header focuses it (also persisted in the URL)
  504 |   await expect(page.getByTestId('herdr-pane-w7:p3')).toBeVisible()
  505 |   await page.getByTestId('herdr-pane-header-w7:p3').click()
  506 |   await expect(page.getByTestId('herdr-pane-w7:p3')).toHaveAttribute('data-focused', 'true')
  507 |   await expect(page).toHaveURL(/pane=w7(:|%3A)p3/)
  508 | 
```