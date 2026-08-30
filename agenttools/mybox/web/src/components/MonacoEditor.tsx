import { useEffect, useMemo, useRef, useState } from 'react'
import Editor, { type OnMount } from '@monaco-editor/react'

interface MonacoEditorProps {
  value: string
  onChange: (value: string) => void
  path?: string
  language?: string
  ariaLabel?: string
  className?: string
  initialLine?: number
  original?: string
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

// computeLineDiff returns the set of modified (current) line numbers that are
// newly added vs the original, and the set that changed in place. Used to draw
// inline diff decorations on a live, editable Monaco editor.
function computeLineDiff(original: string, modified: string) {
  const a = original.split('\n')
  const b = modified.split('\n')
  const n = a.length
  const m = b.length
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1])
    }
  }

  const matchedB = new Set<number>()
  const addedB = new Set<number>()
  const changedB = new Set<number>()
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      matchedB.add(j)
      i++
      j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      // Line a[i] was removed from this position -> the current line changed.
      changedB.add(j)
      i++
    } else {
      addedB.add(j)
      j++
    }
  }
  while (j < m) {
    addedB.add(j)
    j++
  }
  for (let k = 0; k < m; k++) {
    if (!matchedB.has(k) && !addedB.has(k)) changedB.add(k)
  }
  return { added: addedB, changed: changedB }
}

function MonacoEditorInner({
  value,
  onChange,
  path,
  language,
  ariaLabel,
  className,
  initialLine,
  original,
}: MonacoEditorProps) {
  const dark = useIsDark()
  const theme = dark ? 'vs-dark' : 'light'
  const editorRef = useRef<Parameters<OnMount>[0] | null>(null)
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
      const { added, changed } = computeLineDiff(orig, val)
      const mk = (line: number) => ({
        startLineNumber: line,
        startColumn: 1,
        endLineNumber: line,
        endColumn: 1,
      })
      for (const line of added) {
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
      for (const line of changed) {
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

    decorationIds.current = model.deltaDecorations(decorationIds.current, decorations as never[])
  }

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor
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

  return (
    <div className={className}>
      <Editor
        height="60vh"
        language={resolvedLanguage}
        value={value}
        theme={theme}
        onChange={(v) => onChange(v ?? '')}
        onMount={handleMount}
        options={{
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: 'off',
          automaticLayout: true,
          fontSize: 13,
          lineNumbers: 'on',
          tabSize: 2,
          scrollbar: { verticalScrollbarSize: 10, horizontalScrollbarSize: 10 },
          fixedOverflowWidgets: true,
          ariaLabel,
        }}
      />
    </div>
  )
}

export default MonacoEditorInner
