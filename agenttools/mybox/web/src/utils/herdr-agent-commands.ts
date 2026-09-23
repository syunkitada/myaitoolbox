// Slash commands run by sending the command text to the agent like a prompt
// (the running agent interprets /new, /init, ... in its own input line).
export interface AgentCommand {
  label: string
  command: string
}

export const AGENT_COMMANDS: AgentCommand[] = [
  { label: '/new', command: '/new' },
  { label: '/init', command: '/init' },
  { label: '/compact', command: '/compact' },
  { label: '/help', command: '/help' },
]