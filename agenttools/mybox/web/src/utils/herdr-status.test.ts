import { describe, expect, it } from 'vitest'
import {
  createHerdrStatusMemory,
  displayedHerdrAgentStatus,
  updateHerdrStatusMemory,
} from './herdr-status'

const agent = (status: string) => ({ pane_id: 'w1:p1', status })

describe('Herdr unseen completion status', () => {
  it('keeps done visible after Herdr acknowledges it as idle while mybox is unfocused', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('working')], null)
    updateHerdrStatusMemory(memory, [agent('done')], null)
    updateHerdrStatusMemory(memory, [agent('idle')], null)

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('done')
  })

  it('acknowledges the completion when the agent panel receives mybox focus', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('done')], null)
    updateHerdrStatusMemory(memory, [agent('idle')], null)

    expect(displayedHerdrAgentStatus(agent('idle'), memory, 'w1:p1')).toBe('idle')
    updateHerdrStatusMemory(memory, [agent('idle')], 'w1:p1')
    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('idle')
  })

  it('does not make an already viewed completion unread after leaving the panel', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('done')], 'w1:p1')
    updateHerdrStatusMemory(memory, [agent('idle')], 'w1:p1')
    updateHerdrStatusMemory(memory, [agent('idle')], null)

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('idle')
  })

  it('clears an old completion when the agent starts working again', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('done')], null)
    updateHerdrStatusMemory(memory, [agent('idle')], null)
    updateHerdrStatusMemory(memory, [agent('working')], null)

    expect(displayedHerdrAgentStatus(agent('working'), memory, null)).toBe('working')
    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('idle')
  })

  it('tracks a later completion after an earlier one was acknowledged', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('done')], 'w1:p1')
    updateHerdrStatusMemory(memory, [agent('idle')], 'w1:p1')
    updateHerdrStatusMemory(memory, [agent('done')], null)

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('done')
    expect(displayedHerdrAgentStatus(agent('done'), memory, null)).toBe('done')
  })
})
