import { describe, it, expect } from 'vitest'
import { fileAgentName, fileLabel } from './herdr-file-agent'

describe('fileAgentName', () => {
  it('folds non-alphanumeric characters into hyphens', () => {
    expect(fileAgentName('app.go')).toBe('app-go')
    expect(fileAgentName('README.md')).toBe('readme-md')
    expect(fileAgentName('architecture.md')).toBe('architecture-md')
    expect(fileAgentName('src/app.go')).toBe('src-app-go')
    expect(fileAgentName('docs/architecture.md')).toBe('docs-architecture-md')
    expect(fileAgentName('a/b/c.go')).toBe('b-c-go')
    expect(fileAgentName('deep/nested/dir/file.md')).toBe('dir-file-md')
  })

  it('keeps underscore separators', () => {
    expect(fileAgentName('foo_bar.txt')).toBe('foo_bar-txt')
    expect(fileAgentName('a-_-b')).toBe('a-_-b')
  })

  it('lowercases and trims', () => {
    expect(fileAgentName('App.Config.json')).toBe('app-config-json')
    expect(fileAgentName('  My File (final).txt  ')).toBe('my-file-final-txt')
  })

  it('prefixes numbers with f', () => {
    expect(fileAgentName('123.txt')).toBe('f123-txt')
  })

  it('returns empty for unusable filenames', () => {
    expect(fileAgentName('!!!')).toBe('')
    expect(fileAgentName('   ')).toBe('')
  })

  it('caps the name at 32 chars', () => {
    const got = fileAgentName('x'.repeat(40) + '-y'.repeat(10) + '.md')
    expect(got).toHaveLength(32)
    expect(got).toMatch(/^[a-z][a-z0-9_-]*$/)
  })
})

describe('fileLabel', () => {
  it('keeps the filename as the tab label', () => {
    expect(fileLabel('app.go')).toBe('app.go')
  })
})