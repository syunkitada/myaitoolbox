// Quick commands run by sending their text to the agent like a prompt.
export interface AgentCommand {
  id: string
  label: string
  command: string
}

export const AGENT_COMMANDS: AgentCommand[] = [
  { id: 'init', label: '/init', command: '/init' },
  { id: 'compact', label: '/compact', command: '/compact' },
  { id: 'help', label: '/help', command: '/help' },
  { id: 'continue', label: '進めて', command: '進めて' },
  { id: 'next-step', label: '次は何をするとよいですか？', command: '次は何をするとよいですか？' },
]
