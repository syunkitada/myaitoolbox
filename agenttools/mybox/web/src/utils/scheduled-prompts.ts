import type { ScheduledPrompt as ScheduledPromptResponse } from '../api/client'

// Kept only to migrate reservations created by the browser-only implementation
// from older mybox versions. New reservations are stored by the server.
export const LEGACY_SCHEDULED_PROMPTS_STORAGE_KEY = 'mybox.herdr.scheduled-prompts'

export interface ScheduledPrompt {
  id: string
  target: string
  text: string
  scheduledAt: number
}

export function readLegacyScheduledPrompts(): ScheduledPrompt[] {
  try {
    const value: unknown = JSON.parse(window.localStorage.getItem(LEGACY_SCHEDULED_PROMPTS_STORAGE_KEY) ?? '[]')
    if (!Array.isArray(value)) return []
    return value.filter(
      (item): item is ScheduledPrompt =>
        typeof item === 'object' &&
        item !== null &&
        typeof item.id === 'string' &&
        typeof item.target === 'string' &&
        typeof item.text === 'string' &&
        Number.isFinite(item.scheduledAt),
    )
  } catch {
    return []
  }
}

export function clearLegacyScheduledPrompts(): void {
  window.localStorage.removeItem(LEGACY_SCHEDULED_PROMPTS_STORAGE_KEY)
}

let legacyMigration: Promise<ScheduledPrompt[]> | null = null

export function migrateLegacyScheduledPrompts(
  create: (target: string, text: string, scheduledAt: string) => Promise<ScheduledPromptResponse>,
): Promise<ScheduledPrompt[]> {
  if (legacyMigration) return legacyMigration
  const legacy = readLegacyScheduledPrompts()
  if (legacy.length === 0) return Promise.resolve([])
  legacyMigration = (async () => {
    const migrated: ScheduledPrompt[] = []
    const failed: ScheduledPrompt[] = []
    for (const prompt of legacy) {
      if (prompt.scheduledAt <= Date.now()) continue
      try {
        const response = await create(prompt.target, prompt.text, new Date(prompt.scheduledAt).toISOString())
        const converted = fromScheduledPromptResponse(response)
        if (converted) migrated.push(converted)
        else failed.push(prompt)
      } catch {
        failed.push(prompt)
      }
    }
    if (failed.length === 0) {
      clearLegacyScheduledPrompts()
    } else {
      window.localStorage.setItem(LEGACY_SCHEDULED_PROMPTS_STORAGE_KEY, JSON.stringify(failed))
    }
    return migrated
  })().finally(() => {
    legacyMigration = null
  })
  return legacyMigration
}

export function fromScheduledPromptResponse(prompt: ScheduledPromptResponse): ScheduledPrompt | null {
  const timestamp = Date.parse(prompt.scheduled_at)
  if (!Number.isFinite(timestamp)) return null
  return {
    id: prompt.id,
    target: prompt.target,
    text: prompt.text,
    scheduledAt: timestamp,
  }
}

export function formatDateTimeLocal(date: Date): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function formatScheduledAt(timestamp: number): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' }).format(
    new Date(timestamp),
  )
}
