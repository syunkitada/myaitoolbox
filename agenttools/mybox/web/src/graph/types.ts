export type GraphNodeKind = 'directory' | 'file' | 'external'

export type GraphEdgeKind = 'contains' | 'link'

export interface GraphNodeAttributes {
  kind: GraphNodeKind
  label: string
  path: string
  [key: string]: unknown
}

export interface GraphEdgeAttributes {
  kind: GraphEdgeKind
  [key: string]: unknown
}

export type NodeId = string

export interface ExplorerState {
  expandedNodeIds: Set<NodeId>
  selectedNodeId: NodeId | null
}

export interface TreeInputEntry {
  path: string
  kind: 'file' | 'dir'
}

export interface FileTreeNode {
  kind: 'dir' | 'file'
  name: string
  path: string
  markdown: boolean
  children?: FileTreeNode[]
}

export interface ProjectedNode {
  id: NodeId
  kind: GraphNodeKind
  label: string
  path: string
}

export interface ProjectedEdge {
  source: NodeId
  target: NodeId
  kind: GraphEdgeKind
}

export interface LinkReference {
  sourcePath: string
  targetPath: string
}

export interface GraphProjection {
  nodes: ProjectedNode[]
  edges: ProjectedEdge[]
}