export type NavAction = 'open-terminal' | 'new-file' | 'new-task' | 'new-adhoc' | 'open-chat-opencode' | 'open-chat-codex'

const listeners = new Set<(action: NavAction) => void>()

export function subscribeNavActions(fn: (action: NavAction) => void) {
  listeners.add(fn)
  return () => {
    listeners.delete(fn)
  }
}

export function dispatchNavAction(action: NavAction) {
  for (const fn of listeners) fn(action)
}
