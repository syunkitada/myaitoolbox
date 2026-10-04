import { beforeEach, describe, expect, it } from 'vitest'
import {
  acknowledgeHerdrStatusMemory,
  createHerdrStatusMemory,
  displayedHerdrAgentStatus,
  loadHerdrStatusMemory,
  persistHerdrStatusMemory,
  updateHerdrStatusMemory,
} from './herdr-status'

const agent = (status: string) => ({ pane_id: 'w1:p1', status })

describe('Herdr status memory', () => {
  beforeEach(() => localStorage.clear())

  it('marks an idle agent purple after Mybox observed it working', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('idle')])

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('done')
  })

  it('keeps a done agent purple until Mybox focuses it', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('done')])

    expect(displayedHerdrAgentStatus(agent('done'), memory, null)).toBe('done')
    acknowledgeHerdrStatusMemory(memory, [agent('done')], 'w1:p1')
    expect(displayedHerdrAgentStatus(agent('done'), memory, 'w1:p1')).toBe('idle')
    expect(displayedHerdrAgentStatus(agent('done'), memory, null)).toBe('idle')
  })

  it('does not re-notify a completion when Mybox focus leaves', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('idle')])
    acknowledgeHerdrStatusMemory(memory, [agent('idle')], 'w1:p1')

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('idle')
    updateHerdrStatusMemory(memory, [agent('idle')])
    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('idle')
  })

  it('starts a new purple completion after the agent works again', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('idle')])
    acknowledgeHerdrStatusMemory(memory, [agent('idle')], 'w1:p1')
    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('idle')])

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('done')
  })

  it('clears a pending completion when the agent becomes blocked', () => {
    const memory = createHerdrStatusMemory()

    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('blocked')])
    updateHerdrStatusMemory(memory, [agent('idle')])

    expect(displayedHerdrAgentStatus(agent('idle'), memory, null)).toBe('idle')
  })

  it('restores the working and completion memory from localStorage', () => {
    const memory = createHerdrStatusMemory()
    updateHerdrStatusMemory(memory, [agent('working')])
    persistHerdrStatusMemory(memory)

    const restored = loadHerdrStatusMemory()
    updateHerdrStatusMemory(restored, [agent('idle')])

    expect(displayedHerdrAgentStatus(agent('idle'), restored, null)).toBe('done')
  })

  it('restores an acknowledged completion as idle from localStorage', () => {
    const memory = createHerdrStatusMemory()
    updateHerdrStatusMemory(memory, [agent('working')])
    updateHerdrStatusMemory(memory, [agent('idle')])
    acknowledgeHerdrStatusMemory(memory, [agent('idle')], 'w1:p1')
    persistHerdrStatusMemory(memory)

    const restored = loadHerdrStatusMemory()
    updateHerdrStatusMemory(restored, [agent('idle')])

    expect(displayedHerdrAgentStatus(agent('idle'), restored, null)).toBe('idle')
  })
})
