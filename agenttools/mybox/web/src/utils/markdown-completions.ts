import * as monaco from 'monaco-editor'
import { suggestLinkTargets, type LinkPathItem } from './markdown-link-completions'

let currentFile: string | undefined
let currentItems: LinkPathItem[] = []

export function setMarkdownLinkCompletions(filePath: string | undefined, items: LinkPathItem[] | undefined) {
  currentFile = filePath
  currentItems = items ?? []
}

export function provideMarkdownLinkCompletions(model: monaco.editor.ITextModel, position: monaco.Position) {
  const lineUntil = model.getLineContent(position.lineNumber).slice(0, position.column - 1)
  const m = /\[[^\]\n]*\]\(\s*([^)]*)$/.exec(lineUntil)
  if (!m) return { suggestions: [] }
  const typed = m[1]
  const suggestions = suggestLinkTargets(currentFile, currentItems, typed)
  if (suggestions.length === 0) return { suggestions: [] }
  const startCol = Math.max(1, position.column - typed.length)
  const range = new monaco.Range(position.lineNumber, startCol, position.lineNumber, position.column)
  return {
    suggestions: suggestions.map(
      (s): monaco.languages.CompletionItem => ({
        label: s.label,
        kind:
          s.kind === 'dir'
            ? monaco.languages.CompletionItemKind.Folder
            : monaco.languages.CompletionItemKind.File,
        insertText: s.insertText,
        filterText: s.insertText,
        detail: s.detail,
        range,
        sortText: s.sort,
        ...(s.kind === 'dir'
          ? { command: { id: 'editor.action.triggerSuggest', title: '' } as monaco.languages.Command }
          : {}),
      }),
    ),
  }
}

export function registerMarkdownLinkCompletions() {
  monaco.languages.registerCompletionItemProvider('markdown', {
    triggerCharacters: ['(', '[', '.', '/'],
    provideCompletionItems(model, position) {
      if (currentItems.length === 0) return { suggestions: [] }
      return provideMarkdownLinkCompletions(model, position)
    },
  })
}