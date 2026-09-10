# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: app.spec.ts >> sidebar shows workspace status and herdr agents
- Location: tests/e2e/app.spec.ts:633:1

# Error details

```
Error: expect(locator).toContainText(expected) failed

Locator: getByTestId('sidebar-agent-w7:p1')
Expected substring: "OC | stub agent"
Received string:    "proj-dir: opencode"
Timeout: 5000ms

Call log:
  - Expect "toContainText" with timeout 5000ms
  - waiting for getByTestId('sidebar-agent-w7:p1')
    14 × locator resolved to <button data-size="default" data-active="false" data-state="closed" data-sidebar="menu-button" data-slot="sidebar-menu-button" data-testid="sidebar-agent-w7:p1" class="peer/menu-button flex w-full items-center gap-2 overflow-hidden rounded-md p-2 text-left ring-sidebar-ring outline-hidden transition-[width,height,padding] group-has-data-[sidebar=menu-action]/menu-item:pr-8 group-data-[collapsible=icon]:size-8! group-data-[collapsible=icon]:p-2! focus-visible:ring-2 active:bg-sidebar-accent active:text…>…</button>
       - unexpected value "proj-dir: opencode"

```

```yaml
- 'button "proj-dir: opencode agent status working"':
  - text: "proj-dir: opencode"
  - img "agent status working"
```

# Test source

```ts
  549 |   await page.waitForTimeout(2500)
  550 |   await expect(p2Pre).toHaveText(frozen)
  551 |   await toggle.click()
  552 |   await expect(toggle).toHaveAttribute('aria-pressed', 'true')
  553 | 
  554 |   // split an existing pane downward; the new pane appears under the same tab
  555 |   await p2.getByRole('button', { name: 'Split w7:p2 down' }).click()
  556 |   const p4 = page.getByTestId('herdr-pane-w7:p4')
  557 |   await expect(p4).toBeVisible()
  558 | 
  559 |   // pane terminal output is loaded automatically while open
  560 |   await expect(page.locator('.herdr-pane-detail').locator('pre').first()).toContainText(
  561 |     'stub pane output for',
  562 |   )
  563 | 
  564 |   // rename the split pane
  565 |   page.once('dialog', (d) => d.accept('logs'))
  566 |   await p4.getByRole('button', { name: 'Rename', exact: true }).click()
  567 |   await expect(p4).toContainText('logs')
  568 | 
  569 |   // close the pane and then the tab (both confirm dialogs)
  570 |   page.once('dialog', (d) => d.accept())
  571 |   await p4.locator('.herdr-pane-close').click()
  572 |   await expect(p4).toHaveCount(0)
  573 | 
  574 |   page.once('dialog', (d) => d.accept())
  575 |   await newTab.hover()
  576 |   await newTab.getByRole('button', { name: `Close tab w7:t3` }).click()
  577 |   await expect(newTab).toHaveCount(0)
  578 | })
  579 | 
  580 | test('dragging a split divider resizes the neighboring panes', async ({ page }) => {
  581 |   await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  582 |   const ws = page.getByTestId('herdr-workspace-w7')
  583 |   await expect(ws).toBeVisible()
  584 | 
  585 |   // the test owns a fresh tab so it starts from a single full pane
  586 |   page.once('dialog', (d) => d.accept('drag'))
  587 |   await ws.getByRole('button', { name: '+ New Tab' }).click()
  588 |   const tab = page.locator('[data-testid^="herdr-tab-w7:"]').filter({ hasText: 'drag' })
  589 |   await expect(tab).toContainText('drag')
  590 |   const tabId = (await tab.getAttribute('data-testid'))!.replace('herdr-tab-', '')
  591 |   await tab.click()
  592 |   await expect(tab).toHaveAttribute('data-active', 'true')
  593 | 
  594 |   const layout = ws.locator('.herdr-layout')
  595 |   await expect(layout).toBeVisible()
  596 |   const pane = layout.locator('[data-testid^="herdr-pane-"]').first()
  597 |   await expect(pane).toBeVisible()
  598 |   const paneId = (await pane.getAttribute('data-testid'))!.replace('herdr-pane-', '')
  599 | 
  600 |   // split it to the right so a vertical divider appears between the two panes
  601 |   await page.getByRole('button', { name: `Split ${paneId} right` }).click()
  602 |   const divider = layout.locator('[data-testid^="herdr-divider-"]').first()
  603 |   await expect(divider).toBeVisible()
  604 | 
  605 |   const before = (await pane.boundingBox())!.width
  606 |   const db = (await divider.boundingBox())!
  607 | 
  608 |   // drag the divider to the right; the left pane keeps the wider ratio
  609 |   await page.mouse.move(db.x + db.width / 2, db.y + db.height / 2)
  610 |   await page.mouse.down()
  611 |   await page.mouse.move(db.x + db.width / 2 + 120, db.y + db.height / 2, { steps: 24 })
  612 |   await page.mouse.up()
  613 |   await expect
  614 |     .poll(async () => (await pane.boundingBox())?.width, { timeout: 5000 })
  615 |     .toBeGreaterThan(before + 5)
  616 | 
  617 |   // the resize was committed to herdr: a reload keeps the wider split
  618 |   await page.reload()
  619 |   await expect(layout).toBeVisible()
  620 |   const paneAfter = layout.locator('[data-testid^="herdr-pane-"]').first()
  621 |   await expect(paneAfter).toBeVisible()
  622 |   await expect
  623 |     .poll(async () => (await paneAfter.boundingBox())?.width, { timeout: 5000 })
  624 |     .toBeGreaterThan(before + 5)
  625 | 
  626 |   // clean up the tab so later tests start from a predictable server state
  627 |   page.once('dialog', (d) => d.accept())
  628 |   await tab.hover()
  629 |   await tab.getByRole('button', { name: `Close tab ${tabId}` }).click()
  630 |   await expect(tab).toHaveCount(0)
  631 | })
  632 | 
  633 | test('sidebar shows workspace status and herdr agents', async ({ page }) => {
  634 |   await page.goto('/projects/proj/dashboard')
  635 |   const projRow = page
  636 |     .locator('.sidebar-projects button')
  637 |     .filter({ hasText: 'proj' })
  638 |     .first()
  639 |   await expect(
  640 |     projRow.locator('.herdr-workspace-status[aria-label="workspace status working"]'),
  641 |   ).toBeVisible()
  642 | 
  643 |   const agents = page.locator('.sidebar-agents')
  644 |   await expect(agents.locator('[aria-label="agent status working"]').first()).toBeVisible()
  645 |   // agent row details: workspace label, cwd directory name and terminal title
  646 |   const row = page.getByTestId('sidebar-agent-w7:p1')
  647 |   await expect(row).toContainText('proj')
  648 |   await expect(row).toContainText('proj-dir')
> 649 |   await expect(row).toContainText('OC | stub agent')
      |                     ^ Error: expect(locator).toContainText(expected) failed
  650 | })
  651 | 
  652 | test('clicking a sidebar agent opens its operation panel in the herdr tab', async ({ page }) => {
  653 |   await page.goto('/projects/proj/dashboard')
  654 |   await page.getByTestId('sidebar-agent-w7:p1').click()
  655 |   await expect(page).toHaveURL(/\/projects\/proj\/herdr\?agent=w7%3Ap1$/)
  656 |   const detail = page.locator('.herdr-agent-detail')
  657 |   await expect(detail).toBeVisible()
  658 |   await expect(detail.locator('pre')).toContainText('stub output for w7:p1')
  659 | 
  660 |   await page.getByTestId('herdr-prompt-input').fill('hello from sidebar')
  661 |   await detail.getByRole('button', { name: 'Send' }).click()
  662 |   await expect(detail.locator('pre')).toContainText('last prompt: hello from sidebar')
  663 | })
  664 | 
  665 | test('clicking a sidebar agent of another project switches to that project first', async ({
  666 |   page,
  667 | }) => {
  668 |   const row = page.getByTestId('sidebar-agent-w8:p1')
  669 |   await expect(row).toContainText('other')
  670 |   await row.click()
  671 |   await expect(page).toHaveURL(/\/projects\/other\/herdr\?agent=w8%3Ap1$/)
  672 |   const detail = page.locator('.herdr-agent-detail')
  673 |   await expect(detail).toBeVisible()
  674 |   await expect(detail.locator('pre')).toContainText('stub output for w8:p1')
  675 | })
  676 | 
  677 | test('herdr offers a new tab when no tabs or panes exist at all', async ({ page }) => {
  678 |   await page.locator('.project-tabs').getByRole('link', { name: 'Herdr' }).click()
  679 | 
  680 |   // close every visible tab; the last tab of a workspace also removes the
  681 |   // workspace. (Workspaces of other projects are not shown on this page and
  682 |   // do not count as tabs/panes of this project.)
  683 |   for (const tabId of ['w7:t1', 'w7:t2']) {
  684 |     const tab = page.getByTestId(`herdr-tab-${tabId}`)
  685 |     page.once('dialog', (d) => d.accept())
  686 |     await tab.hover()
  687 |     await tab.getByRole('button', { name: `Close tab ${tabId}` }).click()
  688 |     await expect(tab).toHaveCount(0)
  689 |   }
  690 | 
  691 |   // with nothing left, the empty state still offers tab creation
  692 |   const empty = page.getByTestId('herdr-no-workspace')
  693 |   await expect(empty).toContainText(`No herdr workspace found for project "proj"`)
  694 |   await expect(page.getByTestId('herdr-create-first-tab')).toBeVisible()
  695 | 
  696 |   page.once('dialog', (d) => d.accept('fresh'))
  697 |   await page.getByTestId('herdr-create-first-tab').click()
  698 | 
  699 |   // bootstrapping creates the project's first workspace with the named tab
  700 |   const workspace = page.locator('[data-testid^="herdr-workspace-"]')
  701 |   await expect(workspace).toHaveCount(1)
  702 |   await expect(workspace).toContainText('proj')
  703 |   await expect(page.locator('[data-testid^="herdr-tab-"]').filter({ hasText: 'fresh' })).toBeVisible()
  704 | })
  705 | 
  706 | test('graph tab shows the explorer and graph view', async ({ page }) => {
  707 |   await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  708 |   await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
  709 | 
  710 |   const explorer = page.locator('.explorer-pane')
  711 |   await expect(explorer.getByText('README.md').first()).toBeVisible()
  712 | 
  713 |   const canvas = page.locator('.graph-canvas')
  714 |   await expect(canvas).toBeVisible()
  715 |   await expect(page.getByText(/nodes · .* edges/)).toBeVisible()
  716 | 
  717 |   // expanding a directory surfaces its children in both explorer and graph
  718 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  719 |   await expect(explorer.getByRole('button', { name: 'Collapse knowledge' }).first()).toBeVisible()
  720 |   await expect(explorer.getByText('index.md').first()).toBeVisible()
  721 | })
  722 | 
  723 | test('graph explores a file by double-clicking it in the explorer', async ({ page }) => {
  724 |   await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  725 |   const explorer = page.locator('.explorer-pane')
  726 |   await expect(explorer.getByText('README.md').first()).toBeVisible()
  727 |   await explorer.getByText('README.md').first().dblclick()
  728 |   await expect(page).toHaveURL(/\/dashboard\/files\/README\.md$/)
  729 |   await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  730 | })
  731 | 
  732 | test('graph restores the previous explorer state when returning to the tab', async ({ page }) => {
  733 |   await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  734 |   await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
  735 | 
  736 |   const explorer = page.locator('.explorer-pane')
  737 |   await explorer.getByRole('button', { name: 'Expand knowledge' }).click()
  738 |   await expect(explorer.getByRole('button', { name: 'Collapse knowledge' }).first()).toBeVisible()
  739 | 
  740 |   // switch away and back
  741 |   await page.locator('.project-tabs').getByRole('link', { name: 'Files' }).click()
  742 |   await expect(page.getByRole('heading', { name: 'Files', level: 1 })).toBeVisible()
  743 |   await page.locator('.project-tabs').getByRole('link', { name: 'Graph' }).click()
  744 |   await expect(page.getByRole('heading', { name: 'Graph' })).toBeVisible()
  745 | 
  746 |   // the expanded directory is still expanded, its file still listed
  747 |   await expect(explorer.getByRole('button', { name: 'Collapse knowledge' }).first()).toBeVisible()
  748 |   await expect(explorer.getByText('index.md').first()).toBeVisible()
  749 | })
```