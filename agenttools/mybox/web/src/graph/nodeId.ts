import type { GraphNodeKind, NodeId } from './types'

const DIR_PREFIX = 'dir:'
const FILE_PREFIX = 'file:'
const EXT_PREFIX = 'ext:'

export const ROOT_NODE_ID = `${DIR_PREFIX}`

export function dirId(path: string): NodeId {
  return `${DIR_PREFIX}${path}`
}

export function fileId(path: string): NodeId {
  return `${FILE_PREFIX}${path}`
}

export function externalId(path: string): NodeId {
  return `${EXT_PREFIX}${path}`
}

export function isDirId(id: NodeId): boolean {
  return id.startsWith(DIR_PREFIX)
}

export function isFileId(id: NodeId): boolean {
  return id.startsWith(FILE_PREFIX)
}

export function isExternalId(id: NodeId): boolean {
  return id.startsWith(EXT_PREFIX)
}

export function nodeKindOf(id: NodeId): GraphNodeKind | null {
  if (isDirId(id)) return 'directory'
  if (isFileId(id)) return 'file'
  if (isExternalId(id)) return 'external'
  return null
}

export function nodePathOf(id: NodeId): string {
  if (isDirId(id)) return id.slice(DIR_PREFIX.length)
  if (isFileId(id)) return id.slice(FILE_PREFIX.length)
  if (isExternalId(id)) return id.slice(EXT_PREFIX.length)
  return id
}

function parentOf(path: string): string {
  const i = path.lastIndexOf('/')
  return i >= 0 ? path.slice(0, i) : ''
}

export function siblingsOf(id: NodeId): NodeId[] {
  const prefix = isDirId(id) ? DIR_PREFIX : isFileId(id) ? FILE_PREFIX : EXT_PREFIX
  const path = id.slice(prefix.length)
  const parent = parentOf(path)
  const dir = dirId(parent)
  const result: NodeId[] = []
  if (dir !== id) result.push(dir)
  return result
}

export function isAncestorId(ancestor: NodeId, descendant: NodeId): boolean {
  const a = nodePathOf(ancestor)
  const d = nodePathOf(descendant)
  if (a === '') return d !== ''
  return d.startsWith(`${a}/`)
}

export function ancestorsOf(id: NodeId): NodeId[] {
  const prefix = isDirId(id) ? DIR_PREFIX : isFileId(id) ? FILE_PREFIX : EXT_PREFIX
  const path = id.slice(prefix.length)
  const result: NodeId[] = []
  let cur = parentOf(path)
  do {
    result.push(dirId(cur))
    if (cur === '') break
    cur = parentOf(cur)
  } while (true)
  return result
}