import { dirId, fileId, ROOT_NODE_ID } from './nodeId'
import type {
  ExplorerState,
  FileTreeNode,
  GraphProjection,
  NodeId,
  ProjectedNode,
  TreeInputEntry,
} from './types'

const MARKDOWN = /\.(md|markdown)$/i

export function isMarkdownFile(path: string): boolean {
  return MARKDOWN.test(path)
}

export function buildFileTree(list: TreeInputEntry[]): FileTreeNode[] {
  const sorted = [...list].sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === 'dir' ? -1 : 1
    return a.path.localeCompare(b.path)
  })

  const root: FileTreeNode[] = []
  for (const e of sorted) {
    const parts = e.path.split('/')
    let cur = root
    let prefix = ''
    for (let i = 0; i < parts.length - 1; i++) {
      const part = parts[i]
      prefix = prefix ? `${prefix}/${part}` : part
      let dir = cur.find((n): n is FileTreeNode => n.kind === 'dir' && n.name === part)
      if (!dir) {
        dir = { kind: 'dir', name: part, path: prefix, markdown: false, children: [] }
        cur.push(dir)
      }
      cur = dir.children!
    }
    if (e.kind === 'dir') {
      const leaf = parts[parts.length - 1]
      const path = prefix ? `${prefix}/${leaf}` : leaf
      const existing = cur.find((n) => n.path === e.path && n.kind === 'dir')
      if (existing) existing.markdown = false
      else cur.push({ kind: 'dir', name: leaf, path, markdown: false, children: [] })
    } else {
      cur.push({
        kind: 'file',
        name: parts[parts.length - 1],
        path: e.path,
        markdown: isMarkdownFile(e.path),
      })
    }
  }
  return root
}

export function allFilePaths(tree: FileTreeNode[]): string[] {
  const out: string[] = []
  const walk = (nodes: FileTreeNode[]) => {
    for (const n of nodes) {
      if (n.kind === 'file') out.push(n.path)
      else walk(n.children ?? [])
    }
  }
  walk(tree)
  return out
}

/**
 * Collects the directory ids of the subtree rooted at `dirPath`, including the
 * directory itself. Returns an empty array when the directory is not found.
 */
export function subtreeDirIds(tree: FileTreeNode[], dirPath: string): NodeId[] {
  const find = (nodes: FileTreeNode[]): FileTreeNode | null => {
    for (const n of nodes) {
      if (n.kind !== 'dir') continue
      if (n.path === dirPath) return n
      const match = find(n.children ?? [])
      if (match) return match
    }
    return null
  }
  const root = find(tree)
  if (!root) return []
  const out: NodeId[] = []
  const walk = (node: FileTreeNode) => {
    if (node.kind !== 'dir') return
    out.push(dirId(node.path))
    for (const child of node.children ?? []) walk(child)
  }
  walk(root)
  return out
}

/**
 * Visible-tree projection.
 *
 * The graph shows exactly the nodes the Explorer has "opened": a directory's
 * children only become visible when that directory is expanded. The virtual
 * root (top-level entries) is always expanded.
 */
export function computeProjection(
  tree: FileTreeNode[],
  state: ExplorerState,
): { nodes: ProjectedNode[]; edges: GraphProjection['edges'] } {
  const expanded = state.expandedNodeIds
  const nodes: ProjectedNode[] = []
  const edges: GraphProjection['edges'] = []

  const pushNode = (n: ProjectedNode) => nodes.push(n)
  const contains = (source: NodeId, target: NodeId) => edges.push({ source, target, kind: 'contains' })

  const walk = (children: FileTreeNode[], parentDirId: NodeId) => {
    for (const child of children) {
      if (child.kind === 'dir') {
        const id = dirId(child.path)
        pushNode({ id, kind: 'directory', label: child.name, path: child.path })
        contains(parentDirId, id)
        if (expanded.has(id)) walk(child.children ?? [], id)
      } else if (child.markdown) {
        const id = fileId(child.path)
        pushNode({ id, kind: 'file', label: child.name, path: child.path })
        contains(parentDirId, id)
      }
    }
  }

  walk(tree, ROOT_NODE_ID)
  return { nodes, edges }
}

export function visibleMarkdownPaths(tree: FileTreeNode[], state: ExplorerState): string[] {
  return computeProjection(tree, state)
    .nodes.filter((n) => n.kind === 'file')
    .map((n) => n.path)
}

export function visibleNodeIds(projection: { nodes: ProjectedNode[] }): Set<NodeId> {
  return new Set(projection.nodes.map((n) => n.id))
}