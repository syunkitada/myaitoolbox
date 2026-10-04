export const AGENT_OUTPUT_DISPLAY_MODES = ['auto', 'herdr'] as const

export type AgentOutputDisplayMode = (typeof AGENT_OUTPUT_DISPLAY_MODES)[number]

export function isAgentOutputDisplayMode(value: unknown): value is AgentOutputDisplayMode {
  return value === 'auto' || value === 'herdr'
}

/**
 * Normalizes agent output for the compact Agent sidebar without changing the
 * output stored or returned by Herdr.
 */
export function normalizeAgentOutput(text: string): string {
  const lines = text
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((line) => line.replace(/[ \t]+$/g, ''))

  const firstContent = lines.findIndex((line) => line.trim() !== '')
  if (firstContent === -1) return ''

  let lastContent = lines.length - 1
  while (lastContent >= firstContent && lines[lastContent].trim() === '') lastContent -= 1
  const normalized: string[] = []
  let previousWasBlank = false

  for (const line of lines.slice(firstContent, lastContent + 1)) {
    const isBlank = line.trim() === ''
    if (isBlank) {
      if (previousWasBlank) continue
      normalized.push('')
    } else {
      normalized.push(line)
    }
    previousWasBlank = isBlank
  }

  return normalized.join('\n')
}
