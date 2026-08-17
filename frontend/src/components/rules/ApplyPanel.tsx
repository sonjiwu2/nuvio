import { AlertTriangle, Check, Undo2 } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../primitives/Button'
import { Panel } from '../primitives/Panel'
import { ProgressBar } from '../primitives/ProgressBar'
import { formatCount } from '../../lib/format'
import { useApply } from '../../lib/useApply'
import type { ConflictPolicy, MoveRequestItem } from '../../lib/operationTypes'
import type { PreviewEntry } from '../../lib/ruleTypes'
import './ApplyPanel.css'

interface ApplyPanelProps {
  entries: PreviewEntry[]
}

export function ApplyPanel({ entries }: ApplyPanelProps) {
  const { state, apply, cancelApply, undo } = useApply()
  const { status, progress, result, error, undoStatus, undoResult, undoError } = state
  const [policy, setPolicy] = useState<ConflictPolicy>('skip')

  async function handleApply() {
    const items: MoveRequestItem[] = entries.map((e) => ({
      source: e.sourcePath,
      destination: e.destinationPath,
    }))
    await apply(items, policy)
  }

  if (status === 'idle') {
    return (
      <Panel
        title="Apply these changes"
        description="This actually moves files — review the list above first."
      >
        <div className="apply-panel-policy">
          <span className="apply-panel-policy-label">
            If a file already exists at the destination:
          </span>
          <label className="apply-panel-radio">
            <input
              type="radio"
              name="conflict-policy"
              checked={policy === 'skip'}
              onChange={() => setPolicy('skip')}
            />
            Skip it, leave both files as they are
          </label>
          <label className="apply-panel-radio">
            <input
              type="radio"
              name="conflict-policy"
              checked={policy === 'keep_both'}
              onChange={() => setPolicy('keep_both')}
            />
            Keep both — rename the incoming file
          </label>
        </div>
        <p className="apply-panel-confirm-note">
          Nuvio will move {formatCount(entries.length)} {entries.length === 1 ? 'file' : 'files'}.
          You can undo this afterward.
        </p>
        <Button variant="primary" onClick={() => void handleApply()}>
          Move {formatCount(entries.length)} {entries.length === 1 ? 'file' : 'files'}
        </Button>
      </Panel>
    )
  }

  if (status === 'applying') {
    return (
      <Panel title="Moving files…">
        <div className="apply-panel-progress">
          <ProgressBar
            percent={
              progress ? (progress.completed / Math.max(progress.total, 1)) * 100 : undefined
            }
          />
          <div className="apply-panel-progress-stats">
            <span>
              {formatCount(progress?.completed ?? 0)} of{' '}
              {formatCount(progress?.total ?? entries.length)}
            </span>
            {progress?.currentPath && (
              <span className="apply-panel-current-path">{progress.currentPath}</span>
            )}
          </div>
        </div>
        <Button variant="secondary" onClick={cancelApply}>
          Cancel
        </Button>
      </Panel>
    )
  }

  if (status === 'failed') {
    return (
      <Panel title="Apply failed">
        <div className="apply-panel-summary apply-panel-summary--error">
          <AlertTriangle size={16} strokeWidth={2} aria-hidden="true" />
          {error ?? 'An unknown error occurred.'}
        </div>
      </Panel>
    )
  }

  // completed or cancelled
  return (
    <Panel title={status === 'cancelled' ? 'Move cancelled' : 'Move complete'}>
      <div className="apply-panel-summary">
        <Check size={16} strokeWidth={2} aria-hidden="true" />
        {formatCount(result?.succeeded ?? 0)} moved, {formatCount(result?.skipped ?? 0)} skipped,{' '}
        {formatCount(result?.failed ?? 0)} failed
      </div>

      {undoStatus === 'idle' && (result?.succeeded ?? 0) > 0 && (
        <Button variant="secondary" onClick={() => void undo()}>
          <Undo2 size={15} strokeWidth={2} aria-hidden="true" />
          Undo
        </Button>
      )}
      {undoStatus === 'undoing' && <p className="apply-panel-undo-status">Undoing…</p>}
      {undoStatus === 'done' && (
        <p className="apply-panel-undo-status">
          Restored {formatCount(undoResult?.restored ?? 0)} file
          {(undoResult?.restored ?? 0) === 1 ? '' : 's'}
          {(undoResult?.skipped ?? 0) > 0
            ? ` — ${formatCount(undoResult?.skipped ?? 0)} couldn't be safely restored (something changed since the move)`
            : ''}
        </p>
      )}
      {undoStatus === 'failed' && (
        <p className="apply-panel-undo-status apply-panel-undo-status--error">
          Undo failed: {undoError ?? 'unknown error'}
        </p>
      )}
    </Panel>
  )
}
