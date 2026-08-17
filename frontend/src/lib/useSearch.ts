import { useCallback, useEffect, useRef, useState } from 'react'
import { CancelSearch, PickFolder, StartSearch } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  SEARCH_EVENTS,
  type SearchCompletedEvent,
  type SearchFailedEvent,
  type SearchMatch,
  type SearchMatchesEvent,
  type SearchProgress,
  type SearchProgressEvent,
  type SearchResult,
} from './searchTypes'

export type SearchStatus = 'idle' | 'searching' | 'completed' | 'cancelled' | 'failed'

export interface SearchState {
  status: SearchStatus
  root: string | null
  query: string | null
  progress: SearchProgress | null
  matches: SearchMatch[]
  result: SearchResult | null
  error: string | null
}

const initialState: SearchState = {
  status: 'idle',
  root: null,
  query: null,
  progress: null,
  matches: [],
  result: null,
  error: null,
}

/** Drives one Search run: folder + query selection, start, streamed matches, cancel. */
export function useSearch() {
  const [state, setState] = useState<SearchState>(initialState)
  const activeSearchId = useRef<string | null>(null)
  // See useScan's identical field for why this exists independent of React
  // state: unmount-cleanup needs to know synchronously whether to cancel.
  const searchInFlight = useRef(false)

  useEffect(() => {
    const unsubscribers = [
      EventsOn(SEARCH_EVENTS.progress, (event: SearchProgressEvent) => {
        if (event.id !== activeSearchId.current) return
        const { id: _id, ...progress } = event
        setState((prev) => ({ ...prev, progress }))
      }),
      EventsOn(SEARCH_EVENTS.matches, (event: SearchMatchesEvent) => {
        if (event.id !== activeSearchId.current) return
        setState((prev) => ({ ...prev, matches: [...prev.matches, ...event.matches] }))
      }),
      EventsOn(SEARCH_EVENTS.completed, (event: SearchCompletedEvent) => {
        if (event.id !== activeSearchId.current) return
        searchInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'completed', progress: null, result }))
      }),
      EventsOn(SEARCH_EVENTS.cancelled, (event: SearchCompletedEvent) => {
        if (event.id !== activeSearchId.current) return
        searchInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'cancelled', progress: null, result }))
      }),
      EventsOn(SEARCH_EVENTS.failed, (event: SearchFailedEvent) => {
        if (event.id !== activeSearchId.current) return
        searchInFlight.current = false
        setState((prev) => ({ ...prev, status: 'failed', progress: null, error: event.error }))
      }),
    ]

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe())
      if (searchInFlight.current && activeSearchId.current) {
        void CancelSearch(activeSearchId.current)
      }
    }
  }, [])

  const pickFolder = useCallback(async (): Promise<string | null> => {
    const path = await PickFolder()
    return path === '' ? null : path
  }, [])

  const startSearch = useCallback(async (root: string, query: string) => {
    setState({
      status: 'searching',
      root,
      query,
      progress: null,
      matches: [],
      result: null,
      error: null,
    })
    try {
      const id = await StartSearch(root, query)
      activeSearchId.current = id
      searchInFlight.current = true
    } catch (err) {
      setState((prev) => ({
        ...prev,
        status: 'failed',
        error: err instanceof Error ? err.message : String(err),
      }))
    }
  }, [])

  const cancelSearch = useCallback(() => {
    if (activeSearchId.current) {
      void CancelSearch(activeSearchId.current)
    }
  }, [])

  return { state, pickFolder, startSearch, cancelSearch }
}
