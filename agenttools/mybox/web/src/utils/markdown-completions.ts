import * as monaco from 'monaco-editor'
import { completionDirectory, suggestLinkTargets, type LinkPathItem } from './markdown-link-completions'

export type MarkdownLinkCompletionLoader = (dir: string) => Promise<LinkPathItem[]>

interface CompletionContext {
  filePath: string | undefined
  items: LinkPathItem[]
  loader: MarkdownLinkCompletionLoader | undefined
  loadedItemsByDir: Map<string, LinkPathItem[]>
  loadingByDir: Map<string, Promise<LinkPathItem[]>>
  registrations: Set<symbol>
  revision: number
}

const contextsByModel = new Map<string, CompletionContext>()

function modelKey(model: monaco.editor.ITextModel): string {
  return model.uri.toString()
}

function mergeItems(context: CompletionContext, items: LinkPathItem[]): LinkPathItem[] {
  const merged = new Map<string, LinkPathItem>()
  for (const item of items) merged.set(`${item.kind}:${item.path}`, item)
  for (const loaded of context.loadedItemsByDir.values()) {
    for (const item of loaded) merged.set(`${item.kind}:${item.path}`, item)
  }
  return [...merged.values()]
}

export function setMarkdownLinkCompletions(
  model: monaco.editor.ITextModel,
  filePath: string | undefined,
  items: LinkPathItem[] | undefined,
  loader?: MarkdownLinkCompletionLoader,
): () => void {
  const key = modelKey(model)
  let context = contextsByModel.get(key)
  if (!context) {
    context = {
      filePath,
      items: [],
      loader,
      loadedItemsByDir: new Map(),
      loadingByDir: new Map(),
      registrations: new Set(),
      revision: 0,
    }
    contextsByModel.set(key, context)
  }
  if (context.filePath !== filePath || context.loader !== loader) {
    context.loadedItemsByDir = new Map()
    context.loadingByDir = new Map()
    context.revision += 1
  }
  context.filePath = filePath
  context.loader = loader
  context.items = mergeItems(context, items ?? [])
  const registration = Symbol()
  context.registrations.add(registration)
  let released = false
  return () => {
    if (released) return
    released = true
    const current = contextsByModel.get(key)
    if (!current) return
    current.registrations.delete(registration)
    if (current.registrations.size === 0) contextsByModel.delete(key)
  }
}

export function clearMarkdownLinkCompletions(model: monaco.editor.ITextModel) {
  contextsByModel.delete(modelKey(model))
}

function parseLinkInput(model: monaco.editor.ITextModel, position: monaco.Position): { typed: string } | null {
  const lineUntil = model.getLineContent(position.lineNumber).slice(0, position.column - 1)
  const m = /\[[^\]\n]*\]\(\s*([^)]*)$/.exec(lineUntil)
  return m ? { typed: m[1] } : null
}

export function provideMarkdownLinkCompletions(model: monaco.editor.ITextModel, position: monaco.Position) {
  const input = parseLinkInput(model, position)
  if (!input) return { suggestions: [] }
  const { typed } = input
  const context = contextsByModel.get(modelKey(model))
  const suggestions = suggestLinkTargets(context?.filePath, context?.items ?? [], typed)
  if (suggestions.length === 0) return { suggestions: [] }
  const startCol = Math.max(1, position.column - typed.length)
  const range = new monaco.Range(position.lineNumber, startCol, position.lineNumber, position.column)
  return {
    suggestions: suggestions.map(
      (s, index): monaco.languages.CompletionItem => ({
        label: s.label,
        preselect: index === 0,
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

export async function provideMarkdownLinkCompletionsLazily(
  model: monaco.editor.ITextModel,
  position: monaco.Position,
) {
  const input = parseLinkInput(model, position)
  if (!input) return { suggestions: [] }

  const context = contextsByModel.get(modelKey(model))
  const loader = context?.loader
  const file = context?.filePath
  const revision = context?.revision
  if (loader) {
    const dir = completionDirectory(file, input.typed)
    let request = context.loadingByDir.get(dir)
    if (!request) {
      request = loader(dir)
        .then((items) => {
          const current = contextsByModel.get(modelKey(model))
          if (current === context && current.revision === revision && current.loader === loader && current.filePath === file) {
            current.loadedItemsByDir.set(dir, items)
            current.items = mergeItems(current, current.items)
          }
          return items
        })
        .catch(() => [])
      context.loadingByDir.set(dir, request)
    }
    await request
  }
  return provideMarkdownLinkCompletions(model, position)
}

export function registerMarkdownLinkCompletions() {
  monaco.languages.registerCompletionItemProvider('markdown', {
    triggerCharacters: ['(', '[', '.', '/'],
    provideCompletionItems(model, position) {
      return provideMarkdownLinkCompletionsLazily(model, position)
    },
  })
}
