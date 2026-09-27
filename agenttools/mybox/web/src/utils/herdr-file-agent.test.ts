import { describe, it, expect } from 'vitest'
import { taskAgentName, taskDirFromPath, filePathForAgent } from './herdr-file-agent'

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
