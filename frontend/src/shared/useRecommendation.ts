import { useEffect, useState } from 'react'
import { api } from '../api'
import { errorMessage } from './format'
import type { Recommendation } from '../types'

const terminalStatuses = new Set(['completed', 'partial', 'blocked', 'failed'])
const pollInterval = 2000

export function isTerminalStatus(status?: string) {
  return terminalStatuses.has(status ?? '')
}

type StreamState = {
  data: Recommendation | null
  error: string
}

export function useRecommendationStream(recommendationID: string): StreamState {
  const [state, setState] = useState<StreamState>({ data: null, error: '' })

  useEffect(() => {
    if (!recommendationID) {
      setState({ data: null, error: '' })

      return
    }

    let cancelled = false
    let source: EventSource | null = null
    let pollTimer: number | undefined

    const stop = () => {
      source?.close()
      source = null
      window.clearTimeout(pollTimer)
    }

    const apply = (next: Recommendation) => {
      if (cancelled) {
        return
      }

      setState({ data: next, error: '' })

      if (isTerminalStatus(next.status)) {
        stop()
      }
    }

    const poll = async () => {
      try {
        const next = await api<Recommendation>(`/recommendations/${recommendationID}`)
        apply(next)

        if (!cancelled && !isTerminalStatus(next.status)) {
          pollTimer = window.setTimeout(() => void poll(), pollInterval)
        }
      } catch (failure) {
        if (!cancelled) {
          setState((current) => ({ ...current, error: errorMessage(failure) }))
        }
      }
    }

    if ('EventSource' in window) {
      source = new EventSource(`/api/v1/recommendations/${recommendationID}/events`, { withCredentials: true })
      source.addEventListener('recommendation', (event) => {
        try {
          apply(JSON.parse((event as MessageEvent<string>).data) as Recommendation)
        } catch {
          setState((current) => ({ ...current, error: 'Не удалось прочитать обновление подбора' }))
        }
      })
      source.addEventListener('error', () => {
        stop()
        void poll()
      })
    } else {
      void poll()
    }

    return () => {
      cancelled = true
      stop()
    }
  }, [recommendationID])

  return state
}
