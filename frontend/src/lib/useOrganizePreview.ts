import { useCallback, useEffect, useRef, useState } from 'react'
import { CancelOrganizePreview, PickFolder, StartOrganizePreview } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  ORGANIZE_PREVIEW_EVENTS,
  type PreviewCompletedEvent,
  type PreviewEntriesEvent,
  type PreviewEntry,
  type PreviewFailedEvent,
  type PreviewProgress,
  type PreviewProgressEvent,
  type PreviewResult,
} from './ruleTypes'

export type PreviewStatus = 'idle' | 'previewing' | 'completed' | 'cancelled' | 'failed'

export interface OrganizePreviewState {
  status: PreviewStatus
  root: string | null
  progress: PreviewProgress | null
  entries: PreviewEntry[]
  result: PreviewResult | null
  error: string | null
}

const initialState: OrganizePreviewState = {
  status: 'idle',
  root: null,
  progress: null,
  entries: [],
  result: null,
  error: null,
}

/** Drives one Organize dry-run preview: folder selection, start, streamed entries, cancel. */
export function useOrganizePreview() {
  const [state, setState] = useState<OrganizePreviewState>(initialState)
  const activePreviewId = useRef<string | null>(null)
  const previewInFlight = useRef(false)

  useEffect(() => {
    const unsubscribers = [
      EventsOn(ORGANIZE_PREVIEW_EVENTS.progress, (event: PreviewProgressEvent) => {
        if (event.id !== activePreviewId.current) return
        const { id: _id, ...progress } = event
        setState((prev) => ({ ...prev, progress }))
      }),
      EventsOn(ORGANIZE_PREVIEW_EVENTS.entries, (event: PreviewEntriesEvent) => {
        if (event.id !== activePreviewId.current) return
        setState((prev) => ({ ...prev, entries: [...prev.entries, ...event.entries] }))
      }),
      EventsOn(ORGANIZE_PREVIEW_EVENTS.completed, (event: PreviewCompletedEvent) => {
        if (event.id !== activePreviewId.current) return
        previewInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'completed', progress: null, result }))
      }),
      EventsOn(ORGANIZE_PREVIEW_EVENTS.cancelled, (event: PreviewCompletedEvent) => {
        if (event.id !== activePreviewId.current) return
        previewInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'cancelled', progress: null, result }))
      }),
      EventsOn(ORGANIZE_PREVIEW_EVENTS.failed, (event: PreviewFailedEvent) => {
        if (event.id !== activePreviewId.current) return
        previewInFlight.current = false
        setState((prev) => ({ ...prev, status: 'failed', progress: null, error: event.error }))
      }),
    ]

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe())
      if (previewInFlight.current && activePreviewId.current) {
        void CancelOrganizePreview(activePreviewId.current)
      }
    }
  }, [])

  const pickFolder = useCallback(async (): Promise<string | null> => {
    const path = await PickFolder()
    return path === '' ? null : path
  }, [])

  const startPreview = useCallback(async (root: string) => {
    setState({ status: 'previewing', root, progress: null, entries: [], result: null, error: null })
    try {
      const id = await StartOrganizePreview(root)
      activePreviewId.current = id
      previewInFlight.current = true
    } catch (err) {
      setState((prev) => ({
        ...prev,
        status: 'failed',
        error: err instanceof Error ? err.message : String(err),
      }))
    }
  }, [])

  const cancelPreview = useCallback(() => {
    if (activePreviewId.current) {
      void CancelOrganizePreview(activePreviewId.current)
    }
  }, [])

  return { state, pickFolder, startPreview, cancelPreview }
}
