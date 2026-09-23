export interface LinkPathItem {
  path: string
  kind: 'file' | 'dir'
}

export interface LinkTargetSuggestion {
  label: string
  insertText: string
  detail: string
  kind: 'file' | 'dir'
  sort: string
}

export function dirname(p: string): string {
  const i = p.lastIndexOf('/')
  return i < 0 ? '' : p.slice(0, i)
}

export function relativePath(fromDir: string, to: string): string {
  if (!fromDir) return to
  const from = fromDir.split('/').filter(Boolean)
  const toParts = to.split('/').filter(Boolean)
  let i = 0
  while (i < from.length && i < toParts.length && from[i] === toParts[i]) i++
  const up = from.length - i
  return (up > 0 ? '../'.repeat(up) : '') + toParts.slice(i).join('/')
}

const LINKABLE_SEGMENT = /^[^()<>\s]+$/

function isLinkable(rel: string): boolean {
  return rel.split('/').every((seg) => LINKABLE_SEGMENT.test(seg))
}

function trimTrailingSlash(s: string): string {
  return s.replace(/\/+$/, '')
}

interface TreeEntry {
  key: string
  fullPath: string
  dir: boolean
}

// buildEntries derives the file tree relative to baseDir, synthesizing parent
// directory entries even when the input contains no explicit directory items.
function buildEntries(items: LinkPathItem[], baseDir: string, exclude?: string): Map<string, TreeEntry[]> {
  const entriesByDir = new Map<string, Map<string, TreeEntry>>()
  const getMap = (dir: string) => {
    let m = entriesByDir.get(dir)
    if (!m) {
      m = new Map()
      entriesByDir.set(dir, m)
    }
    return m
  }
  for (const item of items) {
    if (!item.path || item.path === exclude) continue
    const r = relativePath(baseDir, item.path)
    if (!r) continue
    const parts = r.split('/')
    let prefix = ''
    for (let i = 0; i < parts.length; i++) {
      prefix = prefix ? `${prefix}/${parts[i]}` : parts[i]
      const isLeaf = i === parts.length - 1
      if (isLeaf) {
        getMap(dirname(prefix)).set(prefix, { key: prefix, fullPath: item.path, dir: item.kind === 'dir' })
      } else {
        getMap(dirname(prefix)).set(prefix, { key: prefix, fullPath: prefix, dir: true })
      }
    }
  }
  const out = new Map<string, TreeEntry[]>()
  for (const [dir, m] of entriesByDir) {
    out.set(dir, [...m.values()])
  }
  return out
}

export function suggestLinkTargets(
  filePath: string | undefined,
  items: LinkPathItem[],
  typed: string,
): LinkTargetSuggestion[] {
  const baseDir = filePath ? dirname(filePath) : ''
  const entriesByDir = buildEntries(items, baseDir, filePath)
  const clean = typed.replace(/^\.\//, '')

  let dirPart: string
  let namePart: string
  if (clean === '') {
    dirPart = ''
    namePart = ''
  } else if (clean.endsWith('/')) {
    dirPart = trimTrailingSlash(clean)
    namePart = ''
  } else {
    const slash = clean.lastIndexOf('/')
    dirPart = slash < 0 ? '' : clean.slice(0, slash)
    namePart = slash < 0 ? clean : clean.slice(slash + 1)
  }

  const useDotSlash = typed === '' || typed.startsWith('./')

  const shown = new Map<string, LinkTargetSuggestion>()
  const add = (key: string, fullPath: string, dir: boolean) => {
    if (!key || !isLinkable(key)) return
    const leaf = key.split('/').pop() ?? key
    const base = useDotSlash ? './' : ''
    const label = dir ? `${leaf}/` : leaf
    const parent = leaf === '..'
    shown.set(key, {
      label,
      insertText: `${base}${key}${dir ? '/' : ''}`,
      detail: fullPath,
      kind: dir ? 'dir' : 'file',
      sort: `${parent ? '9' : dir ? '0' : '1'}${key}/`,
    })
  }

  if (namePart === '') {
    const entries = entriesByDir.get(dirPart) ?? []
    for (const e of entries) {
      add(e.key, e.fullPath, e.dir)
    }
  } else {
    for (const [dir, entries] of entriesByDir) {
      if (dir === dirPart || dir.startsWith(dirPart ? `${dirPart}/` : '')) {
        for (const e of entries) {
          const leaf = e.key.split('/').pop() ?? e.key
          if (leaf.toLowerCase().startsWith(namePart.toLowerCase())) {
            add(e.key, e.fullPath, e.dir)
          }
        }
      }
    }
  }

  const out = [...shown.values()].sort((a, b) => (a.sort < b.sort ? -1 : a.sort > b.sort ? 1 : 0))
  return out.slice(0, 200)
}