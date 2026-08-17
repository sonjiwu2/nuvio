import { useCallback, useEffect, useRef, useState } from 'react'
import { CancelOrganizeApply, StartOrganizeApply, UndoBatch } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import {
  APPLY_EVENTS,
  type ApplyCompletedEvent,
  type ApplyFailedEvent,
  type ApplyProgressEvent,
  type ApplyResult,
  type ConflictPolicy,
  type MoveRequestItem,
  type OperationProgress,
  type UndoResult,
} from './operationTypes'

export type ApplyStatus = 'idle' | 'applying' | 'completed' | 'cancelled' | 'failed'
export type UndoStatus = 'idle' | 'undoing' | 'done' | 'failed'

export interface ApplyState {
  status: ApplyStatus
  progress: OperationProgress | null
  result: ApplyResult | null
  error: string | null
  undoStatus: UndoStatus
  undoResult: UndoResult | null
  undoError: string | null
}

const initialState: ApplyState = {
  status: 'idle',
  progress: null,
  result: null,
  error: null,
  undoStatus: 'idle',
  undoResult: null,
  undoError: null,
}

/** Drives one Organize apply-and-optionally-undo cycle. */
export function useApply() {
  const [state, setState] = useState<ApplyState>(initialState)
  const activeBatchId = useRef<string | null>(null)
  const applyInFlight = useRef(false)

  useEffect(() => {
    const unsubscribers = [
      EventsOn(APPLY_EVENTS.progress, (event: ApplyProgressEvent) => {
        if (event.id !== activeBatchId.current) return
        const { id: _id, ...progress } = event
        setState((prev) => ({ ...prev, progress }))
      }),
      EventsOn(APPLY_EVENTS.completed, (event: ApplyCompletedEvent) => {
        if (event.id !== activeBatchId.current) return
        applyInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'completed', progress: null, result }))
      }),
      EventsOn(APPLY_EVENTS.cancelled, (event: ApplyCompletedEvent) => {
        if (event.id !== activeBatchId.current) return
        applyInFlight.current = false
        const { id: _id, ...result } = event
        setState((prev) => ({ ...prev, status: 'cancelled', progress: null, result }))
      }),
      EventsOn(APPLY_EVENTS.failed, (event: ApplyFailedEvent) => {
        if (event.id !== activeBatchId.current) return
        applyInFlight.current = false
        setState((prev) => ({ ...prev, status: 'failed', progress: null, error: event.error }))
      }),
    ]

    return () => {
      unsubscribers.forEach((unsubscribe) => unsubscribe())
      if (applyInFlight.current && activeBatchId.current) {
        void CancelOrganizeApply(activeBatchId.current)
      }
    }
  }, [])

  const apply = useCallback(async (items: MoveRequestItem[], policy: ConflictPolicy) => {
    setState({ ...initialState, status: 'applying' })
    try {
      const id = await StartOrganizeApply(items, policy)
      activeBatchId.current = id
      applyInFlight.current = true
    } catch (err) {
      setState((prev) => ({
        ...prev,
        status: 'failed',
        error: err instanceof Error ? err.message : String(err),
      }))
    }
  }, [])

  const cancelApply = useCallback(() => {
    if (activeBatchId.current) {
      void CancelOrganizeApply(activeBatchId.current)
    }
  }, [])

  const undo = useCallback(async () => {
    if (!activeBatchId.current) return
    setState((prev) => ({ ...prev, undoStatus: 'undoing', undoError: null }))
    try {
      // UndoBatch's Wails-generated return type types `outcome` as a bare
      // string (it can't know about our OperationOutcome union); the
      // runtime value always is one, so this narrows what we already know
      // to be true rather than asserting something new.
      const result = (await UndoBatch(activeBatchId.current)) as unknown as UndoResult
      setState((prev) => ({ ...prev, undoStatus: 'done', undoResult: result }))
    } catch (err) {
      setState((prev) => ({
        ...prev,
        undoStatus: 'failed',
        undoError: err instanceof Error ? err.message : String(err),
      }))
    }
  }, [])

  return { state, apply, cancelApply, undo }
}
