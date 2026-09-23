import { externalId, fileId, nodePathOf } from './nodeId'
import { parseMarkdownLinks, type LinkResolveContext } from './links'
import { computeProjection } from './tree'
import type {
  ExplorerState,
  FileTreeNode,
  GraphProjection,
  ProjectedNode,
} from './types'

export interface ProjectionContext {
  linkContext: LinkResolveContext
}

export interface BuildProjectionInput {
  tree: FileTreeNode[]
  state: ExplorerState
  contents: Map<string, string>
  linkContext: LinkResolveContext
}

export function buildProjection(input: BuildProjectionInput): GraphProjection {
  const { tree, state, contents, linkContext } = input
  const { nodes, edges: containsEdges } = computeProjection(tree, state)
  const edges = [...containsEdges]

  const visibleFilePath = new Set<string>()
  for (const n of nodes) {
    if (n.kind === 'file') visibleFilePath.add(n.path)
  }

  const externalByPath = new Map<string, ProjectedNode>()
  const linkEdges: GraphProjection['edges'] = []

  for (const n of nodes) {
    if (n.kind !== 'file') continue
    const content = contents.get(n.path)
    if (!content) continue
    for (const ref of parseMarkdownLinks(content, n.path, linkContext)) {
      const target = ref.targetPath
      const targetId = visibleFilePath.has(target) ? fileId(target) : externalId(target)
      if (!visibleFilePath.has(target) && !externalByPath.has(target)) {
        externalByPath.set(target, { id: targetId, kind: 'external', label: target, path: target })
      }
      linkEdges.push({ source: n.id, target: targetId, kind: 'link' })
    }
  }

  return {
    nodes: [...nodes, ...externalByPath.values()],
    edges: [...edges, ...linkEdges],
  }
}

export type { LinkResolveContext }
export { parseMarkdownLinks, nodePathOf }