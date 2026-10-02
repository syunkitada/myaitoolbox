import * as monaco from 'monaco-editor'
import { describe, expect, it, vi } from 'vitest'
import {
  provideMarkdownLinkCompletionsLazily,
  setMarkdownLinkCompletions,
} from './markdown-completions'

vi.mock('monaco-editor', () => ({
  Position: class Position {
    constructor(
      readonly lineNumber: number,
      readonly column: number,
    ) {}
  },
  Range: class Range {
    constructor(
      readonly startLineNumber: number,
      readonly startColumn: number,
      readonly endLineNumber: number,
      readonly endColumn: number,
    ) {}
  },
}))

function model(uri: string, line: string): monaco.editor.ITextModel {
  return {
    uri: { toString: () => uri },
    getLineContent: () => line,
  } as unknown as monaco.editor.ITextModel
}

describe('Markdown link completion state', () => {
  it('loads each typed directory using the corresponding model context', async () => {
    const main = model('inmemory://main', '[guide](./')
    const reference = model('inmemory://reference', '[guide](./')
    const loadMain = vi.fn().mockResolvedValue([{ path: 'docs/guide.md', kind: 'file' as const }])
    const loadReference = vi.fn().mockResolvedValue([{ path: 'notes/guide.md', kind: 'file' as const }])

    setMarkdownLinkCompletions(main, 'docs/main.md', [], loadMain)
    setMarkdownLinkCompletions(reference, 'notes/reference.md', [], loadReference)

    await provideMarkdownLinkCompletionsLazily(main, new monaco.Position(1, 10))
    await provideMarkdownLinkCompletionsLazily(reference, new monaco.Position(1, 10))

    expect(loadMain).toHaveBeenCalledWith('docs')
    expect(loadReference).toHaveBeenCalledWith('notes')
  })

  it('keeps a shared URI context until the last editor releases it', async () => {
    const shared = model('inmemory://shared', '[guide](./')
    const load = vi.fn().mockResolvedValue([{ path: 'docs/guide.md', kind: 'file' as const }])
    const releaseMain = setMarkdownLinkCompletions(shared, 'docs/main.md', [], load)
    const releaseReference = setMarkdownLinkCompletions(shared, 'docs/main.md', [], load)

    releaseReference()
    await provideMarkdownLinkCompletionsLazily(shared, new monaco.Position(1, 10))
    expect(load).toHaveBeenCalledTimes(1)

    releaseMain()
    await provideMarkdownLinkCompletionsLazily(shared, new monaco.Position(1, 10))
    expect(load).toHaveBeenCalledTimes(1)
  })

})
