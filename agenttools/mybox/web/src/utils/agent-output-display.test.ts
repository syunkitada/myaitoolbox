import { describe, expect, it } from 'vitest'
import { normalizeAgentOutput } from './agent-output-display'

describe('normalizeAgentOutput', () => {
  it('normalizes line endings, trailing whitespace, and repeated blank lines', () => {
    expect(normalizeAgentOutput('\r\nfirst  \r\n\r\n\r\nsecond\t\n\n')).toBe('first\n\nsecond')
  })

  it('keeps indentation on non-empty lines', () => {
    expect(normalizeAgentOutput('  first\n    second')).toBe('  first\n    second')
  })
})
