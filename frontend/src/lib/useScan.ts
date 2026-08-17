import { useCallback, useEffect, useRef, useState } from 'react'
import { CancelScan, PickFolder, StartScan } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  SCAN_EVENTS,
  type ScanCompletedEvent,
  type ScanFailedEvent,
  type ScanProgress,
  type ScanProgressEvent,
  type ScanResult,
} from './scanTypes'

export type ScanStatus = 'idle' | 'scanning' | 'completed' | 'cancelled' | 'failed'

export interface ScanState {
  status: ScanStatus
  root: string | null
  progress: ScanProgress | null
  result: ScanResult | null
  error: string | null
}

const initialState: ScanState = {
  status: 'idle',
  root: null,
  progress: null,
  result: null,
  error: null,
}

/** Drives one Storage Explorer scan: folder selection, start, live progress, cancel. */
export function useScan() {
  const [state, setState] = useState<ScanState>(initialState)
  const activeScanId = useRef<string | null>(null)
  // Tracks whether activeScanId is still running on the backend, independent
  // of React state, so unmount-cleanup can cancel it without waiting on a
  // render. Without this, navigating away from Storage mid-scan would leave
  // the backend scan running with nothing left able to cancel it.
  const scanInFlight = useRef(false)

  useEffect(() => {
    const unsubscribers = [
      EventsOn(SCAN_EVENTS.progress, (event: ScanProgressEvent) => {
        if (event.id !== activeScanId.current) return
        const { id: _id, ...progress } = event
        setState((prev) => ({ ...prev, progress }))
      }),
      EventsOn(SCAN_EVENTS.completed, (event: ScanCompletedEvent) => {
        if (event.id !== activeScanId.current) return
        scanInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'completed', progress: null, result }))
      }),
      EventsOn(SCAN_EVENTS.cancelled, (event: ScanCompletedEvent) => {
        if (event.id !== activeScanId.current) return
        scanInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'cancelled', progress: null, result }))
      }),
      EventsOn(SCAN_EVENTS.failed, (event: ScanFailedEvent) => {
        if (event.id !== activeScanId.current) return
        scanInFlight.current = false
        setState((prev) => ({ ...prev, status: 'failed', progress: null, error: event.error }))
      }),
    ]

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe())
      if (scanInFlight.current && activeScanId.current) {
        void CancelScan(activeScanId.current)
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
      const id = await StartScan(root)
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
      void CancelScan(activeScanId.current)
    }
  }, [])

  return { state, pickFolder, startScan, cancelScan }
}
