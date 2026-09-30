import { useEffect, useMemo, useRef, useState } from 'react'
import Editor, { type OnMount } from '@monaco-editor/react'
import { api } from '../api/client'
import { computeLineDiff } from '../utils/line-diff'
import { setMarkdownLinkCompletions } from '../utils/markdown-completions'

interface MonacoEditorProps {
  value: string
  onChange: (value: string) => void
  path?: string
  language?: string
  ariaLabel?: string
  className?: string
  height?: string | number
  initialLine?: number
  original?: string
  completions?: Array<{ path: string; kind: 'file' | 'dir' }>
}

const EXT_LANGUAGE: Record<string, string> = {
  ts: 'typescript',
  tsx: 'typescript',
  js: 'javascript',
  jsx: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  json: 'json',
  jsonc: 'json',
  json5: 'json',
  md: 'markdown',
  markdown: 'markdown',
  py: 'python',
  go: 'go',
  rb: 'ruby',
  rs: 'rust',
  java: 'java',
  c: 'c',
  h: 'c',
  cpp: 'cpp',
  cc: 'cpp',
  hpp: 'cpp',
  cs: 'csharp',
  php: 'php',
  sh: 'shell',
  bash: 'shell',
  zsh: 'shell',
  yml: 'yaml',
  yaml: 'yaml',
  toml: 'toml',
  css: 'css',
  scss: 'scss',
  less: 'less',
  html: 'html',
  htm: 'html',
  xml: 'xml',
  svg: 'xml',
  sql: 'sql',
  kt: 'kotlin',
  swift: 'swift',
  dart: 'dart',
  lua: 'lua',
  r: 'r',
}

const NAME_LANGUAGE: Record<string, string> = {
  dockerfile: 'dockerfile',
  makefile: 'plaintext',
  gitignore: 'plaintext',
}

function languageFromPath(path?: string): string | undefined {
  if (!path) return undefined
  const base = path.split(/[\\/]/).pop() ?? path
  const lower = base.toLowerCase()
  if (NAME_LANGUAGE[lower]) return NAME_LANGUAGE[lower]
  const dot = lower.lastIndexOf('.')
  if (dot >= 0) {
    const ext = lower.slice(dot + 1)
    return EXT_LANGUAGE[ext]
  }
  return undefined
}

function relativeLinkPath(fromDir: string, target: string): string {
  const from = fromDir.split('/').filter(Boolean)
  const to = target.split('/').filter(Boolean)
  let common = 0
  while (common < from.length && common < to.length && from[common] === to[common]) common++
  return `${'../'.repeat(from.length - common)}${to.slice(common).join('/')}`
}

function useIsDark() {
  const [dark, setDark] = useState(() => document.documentElement.classList.contains('dark'))
  useEffect(() => {
    const observer = new MutationObserver(() => {
      setDark(document.documentElement.classList.contains('dark'))
    })
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [])
  return dark
}

function MonacoEditorInner({
  value,
  onChange,
  path,
  language,
  ariaLabel,
  className,
  height = '60vh',
  initialLine,
  original,
  completions,
}: MonacoEditorProps) {
  const dark = useIsDark()
  const theme = dark ? 'vs-dark' : 'light'
  const editorRef = useRef<Parameters<OnMount>[0] | null>(null)
  const pathRef = useRef(path)
  pathRef.current = path
  const decorationIds = useRef<string[]>([])
  const valueRef = useRef(value)
  valueRef.current = value
  const originalRef = useRef(original)
  originalRef.current = original

  const resolvedLanguage = useMemo(() => {
    if (language) return language
    return languageFromPath(path) ?? 'plaintext'
  }, [language, path])

  const applyDecorations = () => {
    const editor = editorRef.current
    if (!editor) return
    const model = editor.getModel()
    if (!model) return

    let decorations: {
      range: { startLineNumber: number; startColumn: number; endLineNumber: number; endColumn: number }
      options: { isWholeLine: boolean; className?: string; linesDecorationsClassName?: string; marginClassName?: string }
    }[] = []

    const orig = originalRef.current
    const val = valueRef.current
    if (orig !== undefined && val !== orig) {
      const diff = computeLineDiff(orig, val)
      if (diff) {
        const mk = (line: number) => ({
          startLineNumber: line,
          startColumn: 1,
          endLineNumber: line,
          endColumn: 1,
        })
        for (const line of diff.added) {
          decorations.push({
            range: mk(line),
            options: {
              isWholeLine: true,
              className: 'editor-diff-added',
              linesDecorationsClassName: 'editor-diff-added-line',
              marginClassName: 'editor-diff-added-margin',
            },
          })
        }
        for (const line of diff.changed) {
          decorations.push({
            range: mk(line),
            options: {
              isWholeLine: true,
              className: 'editor-diff-changed',
              marginClassName: 'editor-diff-changed-margin',
            },
          })
        }
      }
    }

    decorationIds.current = model.deltaDecorations(decorationIds.current, decorations as never[])
  }

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor
    const onNativeKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Enter' || resolvedLanguage !== 'markdown') return
      const position = editor.getPosition()
      const model = editor.getModel()
      if (!position || !model) return
      const lineUntil = model.getLineContent(position.lineNumber).slice(0, position.column - 1)
      const match = /\[[^\]\n]*\]\(\s*([^)]*)$/.exec(lineUntil)
      if (!match) return
      const row = document.querySelector<HTMLElement>('.suggest-widget .monaco-list-row.focused')
      const target = row?.querySelector<HTMLElement>('.details-label')?.textContent?.trim()
      if (!row || !target) return
      const typed = match[1]
      const fileDir = pathRef.current?.slice(0, pathRef.current.lastIndexOf('/')) ?? ''
      const prefix = typed.startsWith('./') ? './' : ''
      const isDir = row.getAttribute('aria-label')?.endsWith(', Folder') ?? false
      const insertText = `${prefix}${relativeLinkPath(fileDir, target)}${isDir ? '/' : ''}`
      event.preventDefault()
      event.stopImmediatePropagation()
      editor.trigger('keyboard', 'hideSuggestWidget', {})
      ;(() => {
        const currentPosition = editor.getPosition()
        const currentModel = editor.getModel()
        if (!currentPosition || !currentModel) return
        const currentLine = currentModel.getLineContent(currentPosition.lineNumber)
        const currentMatch = /\[[^\]\n]*\]\(\s*([^)]*)(?=\)|$)/.exec(currentLine)
        const currentPrefix = /\[[^\]\n]*\]\(\s*/.exec(currentLine)
        if (!currentMatch || !currentPrefix) return
        const currentStartColumn = currentPrefix.index + currentPrefix[0].length + 1
        const currentEndColumn = currentStartColumn + currentMatch[1].length
        editor.executeEdits('markdown-link-completion', [
          {
            range: {
              startLineNumber: currentPosition.lineNumber,
              startColumn: currentStartColumn,
              endLineNumber: currentPosition.lineNumber,
              endColumn: currentEndColumn,
            },
            text: insertText,
          },
        ])
        editor.setPosition({ lineNumber: currentPosition.lineNumber, column: currentStartColumn + insertText.length })
        const acceptedValue = currentModel.getValue()
        onChange(acceptedValue)
        window.setTimeout(() => {
          const latestModel = editor.getModel()
          if (latestModel && latestModel.getValue() !== acceptedValue) latestModel.setValue(acceptedValue)
          onChange(acceptedValue)
        }, 0)
        window.setTimeout(() => {
          const latestModel = editor.getModel()
          if (latestModel && latestModel.getValue() !== acceptedValue) latestModel.setValue(acceptedValue)
          onChange(acceptedValue)
        }, 100)
      })()
    }
    window.addEventListener('keydown', onNativeKeyDown, true)
    const onMonacoKeyDown = editor.onKeyDown((event) => {
      if (event.keyCode !== monaco.KeyCode.Enter || resolvedLanguage !== 'markdown') return
      if (!document.querySelector('.suggest-widget .monaco-list-row')) return
      event.preventDefault()
      event.stopPropagation()
    })
    editor.onDidDispose(() => {
      window.removeEventListener('keydown', onNativeKeyDown, true)
      onMonacoKeyDown.dispose()
    })
    if (monaco) {
      try {
        monaco.languages.typescript?.javascriptDefaults?.setDiagnosticsOptions?.({ noSemanticValidation: true })
        monaco.languages.typescript?.typescriptDefaults?.setDiagnosticsOptions?.({ noSemanticValidation: true })
      } catch {
        /* noop */
      }
    }
    if (initialLine && initialLine > 1) {
      const line = Math.min(initialLine, editor.getModel()?.getLineCount() ?? initialLine)
      editor.revealLineInCenter(line)
      editor.setPosition({ lineNumber: line, column: 1 })
    }
    applyDecorations()
  }

  useEffect(() => {
    applyDecorations()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, original])

  useEffect(() => {
    return () => {
      decorationIds.current = []
    }
  }, [])

  useEffect(() => {
    if (resolvedLanguage === 'markdown') {
      setMarkdownLinkCompletions(path, completions)
    }
  }, [path, completions, resolvedLanguage])

  // BrowserPage supplies the already-loaded tree. Keep a fallback for other
  // editor callers, while avoiding a second recursive scan in the browser.
  useEffect(() => {
    if (resolvedLanguage !== 'markdown' || !path || completions !== undefined) return
    let cancelled = false
    void api
      .listFiles({ showHidden: true })
      .then((entries) => {
        if (cancelled) return
        setMarkdownLinkCompletions(
          path,
          entries.map(({ path: entryPath, kind }) => ({ path: entryPath, kind })),
        )
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [completions, path, resolvedLanguage])

  return (
    <div className={className}>
      <Editor
        height={height}
        language={resolvedLanguage}
        value={value}
        theme={theme}
        onChange={(v) => onChange(v ?? '')}
        onMount={handleMount}
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          automaticLayout: true,
          fontSize: 13,
          lineNumbers: 'on',
          tabSize: 2,
          scrollbar: { verticalScrollbarSize: 10, horizontalScrollbarSize: 10 },
          fixedOverflowWidgets: true,
          acceptSuggestionOnEnter: 'off',
          ariaLabel,
          ...(resolvedLanguage === 'markdown'
            ? { quickSuggestions: { other: 'on', comments: 'off', strings: 'on' } }
            : {}),
        }}
      />
    </div>
  )
}

export default MonacoEditorInner
