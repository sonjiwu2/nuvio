import { useCallback, useEffect, useRef, useState } from 'react'
import { CancelDuplicateScan, PickFolder, StartDuplicateScan } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  DUPLICATE_EVENTS,
  type DuplicateCompletedEvent,
  type DuplicateFailedEvent,
  type DuplicateProgress,
  type DuplicateProgressEvent,
  type DuplicateResult,
} from './duplicateTypes'

export type DuplicateScanStatus = 'idle' | 'scanning' | 'completed' | 'cancelled' | 'failed'

export interface DuplicateScanState {
  status: DuplicateScanStatus
  root: string | null
  progress: DuplicateProgress | null
  result: DuplicateResult | null
  error: string | null
}

const initialState: DuplicateScanState = {
  status: 'idle',
  root: null,
  progress: null,
  result: null,
  error: null,
}

/**
 * Drives one duplicate scan. Unlike useScan/useSearch, there is no
 * "entries streaming in" state: a file can't be reported as duplicated
 * until every file has been seen and hashed, so results only exist once
 * the scan is complete.
 */
export function useDuplicates() {
  const [state, setState] = useState<DuplicateScanState>(initialState)
  const activeScanId = useRef<string | null>(null)
  const scanInFlight = useRef(false)

  useEffect(() => {
    const unsubscribers = [
      EventsOn(DUPLICATE_EVENTS.progress, (event: DuplicateProgressEvent) => {
        if (event.id !== activeScanId.current) return
        const { id: _id, ...progress } = event
        setState((prev) => ({ ...prev, progress }))
      }),
      EventsOn(DUPLICATE_EVENTS.completed, (event: DuplicateCompletedEvent) => {
        if (event.id !== activeScanId.current) return
        scanInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'completed', progress: null, result }))
      }),
      EventsOn(DUPLICATE_EVENTS.cancelled, (event: DuplicateCompletedEvent) => {
        if (event.id !== activeScanId.current) return
        scanInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'cancelled', progress: null, result }))
      }),
      EventsOn(DUPLICATE_EVENTS.failed, (event: DuplicateFailedEvent) => {
        if (event.id !== activeScanId.current) return
        scanInFlight.current = false
        setState((prev) => ({ ...prev, status: 'failed', progress: null, error: event.error }))
      }),
    ]

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe())
      if (scanInFlight.current && activeScanId.current) {
        void CancelDuplicateScan(activeScanId.current)
      }
    }
  }, [])

  const pickFolder = useCallback(async (): Promise<string | null> => {
    const path = await PickFolder()
    return path === '' ? null : path
  }, [])

  const startScan = useCallback(async (root: string) => {
    setState({ status: 'scanning', root, progress: null, result: null, error: null })
    try {
      const id = await StartDuplicateScan(root)
      activeScanId.current = id
      scanInFlight.current = true
    } catch (err) {
      setState((prev) => ({
        ...prev,
        status: 'failed',
        error: err instanceof Error ? err.message : String(err),
      }))
    }
  }, [])

  const cancelScan = useCallback(() => {
    if (activeScanId.current) {
      void CancelDuplicateScan(activeScanId.current)
    }
  }, [])

  return { state, pickFolder, startScan, cancelScan }
}
