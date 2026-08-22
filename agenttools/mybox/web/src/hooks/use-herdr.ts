import { useCallback, useEffect, useRef, useState } from 'react'
import { api, HerdrOverview } from '../api/client'

export interface HerdrState {
  overview: HerdrOverview | null
  error: string | null
  loading: boolean
}

export function useHerdrOverview(intervalMs = 5000) {
  const [state, setState] = useState<HerdrState>({ overview: null, error: null, loading: true })
  const seq = useRef(0)

  const refresh = useCallback(async () => {
    const mySeq = ++seq.current
    try {
      const overview = await api.getHerdrOverview()
      // Drop stale responses that resolve after a newer refresh was started.
      if (mySeq !== seq.current) return
      setState({ overview, error: null, loading: false })
    } catch (e) {
      if (mySeq !== seq.current) return
      setState((prev) => ({
        overview: prev.overview,
        error: e instanceof Error ? e.message : String(e),
        loading: false,
      }))
    }
  }, [])

  useEffect(() => {
    void refresh()
    const id = setInterval(() => {
      if (!document.hidden) void refresh()
    }, intervalMs)
    const onVisible = () => {
      if (!document.hidden) void refresh()
    }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(id)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [refresh, intervalMs])

  return { ...state, refresh }
}
