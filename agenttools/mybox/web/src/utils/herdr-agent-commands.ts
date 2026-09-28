// Quick commands run by sending their text to the agent like a prompt.
export interface AgentCommand {
  id: string
  label: string
  command: string
}

export const AGENT_COMMANDS: AgentCommand[] = [
  { id: 'new', label: '/new', command: '/new' },
  { id: 'compact', label: '/compact', command: '/compact' },
  { id: 'help', label: '/help', command: '/help' },
  { id: 'resume', label: '/resume', command: '/resume' },
  { id: 'plan', label: '/plan', command: '/plan' },
  { id: 'status', label: '/status', command: '/status' },
  { id: 'continue', label: '進めて', command: '進めて' },
  { id: 'next-step', label: '次は何をするとよいですか？', command: '次は何をするとよいですか？' },
]
