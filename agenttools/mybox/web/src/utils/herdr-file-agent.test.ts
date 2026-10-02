import { describe, it, expect } from 'vitest'
import {
  filePathForAgent,
  linkedTaskAgentForPath,
  linkedTaskAgentForTab,
  linkedTaskRenameForLabel,
  linkedTaskRenameForPathMove,
  taskAgentName,
  taskDirFromPath,
  taskDirectoryPathForLabel,
  taskFilePathForAgent,
} from './herdr-file-agent'

describe('taskDirFromPath', () => {
  it('returns the task directory for files under _tasks/<dir>/', () => {
    expect(taskDirFromPath('_tasks/20260919_foo/task.md')).toBe('20260919_foo')
    expect(taskDirFromPath('_tasks/20260919_foo/notes/design.md')).toBe('20260919_foo')
  })

  it('returns null outside a task directory', () => {
    expect(taskDirFromPath('src/app.go')).toBeNull()
    expect(taskDirFromPath('README.md')).toBeNull()
    expect(taskDirFromPath('notes/idea.md')).toBeNull()
    expect(taskDirFromPath('tasks.md')).toBeNull()
    expect(taskDirFromPath('tasks')).toBeNull()
    expect(taskDirFromPath('_tasks/foo/../evil.md')).toBeNull()
  })
})

describe('taskAgentName', () => {
  it('folds the task directory into an agent name', () => {
    expect(taskAgentName('_tasks/20260919_foo/task.md')).toBe('f20260919_foo')
    expect(taskAgentName('_tasks/foo/notes.md')).toBe('foo')
    expect(taskAgentName('_tasks/My-Task/a.md')).toBe('my-task')
  })

  it('keeps underscore separators', () => {
    expect(taskAgentName('_tasks/foo_bar/a.txt')).toBe('foo_bar')
    expect(taskAgentName('_tasks/a-_-b/c.md')).toBe('a-_-b')
  })

  it('prefixes digit-leading names with f', () => {
    expect(taskAgentName('_tasks/20260919_foo/task.md')).toBe('f20260919_foo')
    expect(taskAgentName('_tasks/123/x.md')).toBe('f123')
  })

  it('returns empty for paths outside a task directory', () => {
    expect(taskAgentName('app.go')).toBe('')
    expect(taskAgentName('   ')).toBe('')
  })

  it('caps the name at 32 chars', () => {
    const got = taskAgentName('_tasks/' + 'x'.repeat(40) + '-y'.repeat(10) + '/task.md')
    expect(got).toHaveLength(32)
    expect(got).toMatch(/^[a-z][a-z0-9_-]*$/)
  })
})

describe('filePathForAgent', () => {
  const files = [
    { path: '_tasks/20260919_foo/task.md' },
    { path: '_tasks/20260919_foo/notes.md' },
    { path: '_tasks/20260919_bar/task.md' },
    { path: 'src/app.go' },
  ]

  it('prefers the canonical task.md of the matching task directory', () => {
    expect(filePathForAgent(files, 'f20260919_foo')).toBe('_tasks/20260919_foo/task.md')
    expect(filePathForAgent(files, 'f20260919_bar')).toBe('_tasks/20260919_bar/task.md')
  })

  it('returns null when no file matches', () => {
    expect(filePathForAgent(files, 'zzz-zzz')).toBeNull()
  })

  it('returns null for an empty file list', () => {
    expect(filePathForAgent([], 'f20260919_foo')).toBeNull()
  })
})

describe('taskFilePathForAgent', () => {
  const agent = { name: 'f20260919_foo', pane_id: 'w1:p1' }
  const panes = [{ pane_id: 'w1:p1', tab_id: 'w1:t1' }]
  const tabs = [{ tab_id: 'w1:t1', label: '20260919_foo' }]

  it('resolves the task file from the agent pane tab label', () => {
    expect(taskFilePathForAgent(agent, panes, tabs)).toBe('_tasks/20260919_foo/task.md')
  })

  it('returns null when the pane or task tab cannot be resolved', () => {
    expect(taskFilePathForAgent({ name: agent.name, pane_id: 'missing' }, panes, tabs)).toBeNull()
    expect(taskFilePathForAgent(agent, panes, [{ tab_id: 'w1:t1', label: 'shell' }])).toBeNull()
  })
})

describe('linked task agent helpers', () => {
  const agent = { name: 'f20260919_foo', custom_name: 'f20260919_foo', pane_id: 'w1:p1' }
  const panes = [{ pane_id: 'w1:p1', tab_id: 'w1:t1' }]
  const tabs = [{ tab_id: 'w1:t1', label: '20260919_foo' }]

  it('accepts task-directory labels and rejects labels unsupported by file agents', () => {
    expect(taskDirectoryPathForLabel('20260919_foo')).toBe('_tasks/20260919_foo')
    expect(taskDirectoryPathForLabel('has space')).toBeNull()
    expect(taskDirectoryPathForLabel('nested/name')).toBeNull()
  })

  it('resolves the linked task from a tab or a path inside its directory', () => {
    const link = linkedTaskAgentForTab('w1:t1', [agent], [
      { pane_id: 'w1:p2', tab_id: 'w1:t1' },
      ...panes,
    ], tabs)
    expect(link?.taskDirectory).toBe('_tasks/20260919_foo')
    expect(link?.taskFile).toBe('_tasks/20260919_foo/task.md')
    expect(link?.pane.pane_id).toBe('w1:p1')
    expect(linkedTaskAgentForPath('_tasks/20260919_foo/notes.md', [agent], [
      { pane_id: 'w1:p2', tab_id: 'w1:t1' },
      ...panes,
    ], tabs)?.tab.tab_id).toBe('w1:t1')
    expect(linkedTaskAgentForPath('_tasks/other/notes.md', [agent], panes, tabs)).toBeNull()
  })

  it('selects the matching file agent when another agent shares the same tab', () => {
    const otherAgent = { name: 'other', custom_name: 'other', pane_id: 'w1:p2' }
    const link = linkedTaskAgentForTab('w1:t1', [otherAgent, agent], [
      { pane_id: 'w1:p2', tab_id: 'w1:t1' },
      ...panes,
    ], tabs)
    expect(link?.agent.pane_id).toBe('w1:p1')
  })

  it('builds synchronized rename plans for tab and directory changes', () => {
    const link = linkedTaskAgentForTab('w1:t1', [agent], panes, tabs)
    expect(link).not.toBeNull()
    expect(linkedTaskRenameForLabel(link!, '20260920_bar')).toEqual({
      oldTaskDirectory: '_tasks/20260919_foo',
      newTaskDirectory: '_tasks/20260920_bar',
      oldAgentName: 'f20260919_foo',
      newAgentName: 'f20260920_bar',
      label: '20260920_bar',
    })
    expect(linkedTaskRenameForLabel(link!, 'has space')).toBeNull()
    expect(linkedTaskRenameForPathMove(link!, '_tasks/20260920_bar/task.md')).toMatchObject({
      newTaskDirectory: '_tasks/20260920_bar',
      newAgentName: 'f20260920_bar',
    })
  })
})
