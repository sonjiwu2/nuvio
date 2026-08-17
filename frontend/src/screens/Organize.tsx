import { AlertCircle, FolderOpen, ShieldCheck } from 'lucide-react'
import { Button } from '../components/primitives/Button'
import { EmptyState } from '../components/primitives/EmptyState'
import { Panel } from '../components/primitives/Panel'
import { ProgressBar } from '../components/primitives/ProgressBar'
import { PreviewEntryList } from '../components/rules/PreviewEntryList'
import { formatBytes, formatCount } from '../lib/format'
import type { ScreenKey } from '../lib/navigation'
import { useOrganizePreview } from '../lib/useOrganizePreview'
import { useRules } from '../lib/useRules'
import './Organize.css'

interface OrganizeProps {
  onNavigate: (key: ScreenKey) => void
}

export function Organize({ onNavigate }: OrganizeProps) {
  const { rules, loading: rulesLoading } = useRules()
  const { state, pickFolder, startPreview, cancelPreview } = useOrganizePreview()
  const { status, root, progress, entries, result, error } = state

  async function handleChooseFolder() {
    const path = await pickFolder()
    if (path) await startPreview(path)
  }

  async function handleRePreview() {
    if (root) await startPreview(root)
  }

  return (
    <div className="organize-screen">
      <header className="organize-header">
        <div>
          <h1 className="organize-title">Organize</h1>
          <p className="organize-subtitle">Preview what your rules would do to a folder.</p>
        </div>
        {!rulesLoading && rules.length > 0 && status !== 'previewing' && (
          <div className="organize-actions">
            {root && (
              <Button variant="secondary" onClick={() => void handleRePreview()}>
                Preview again
              </Button>
            )}
            <Button variant="primary" onClick={() => void handleChooseFolder()}>
              <FolderOpen size={15} strokeWidth={2} aria-hidden="true" />
              Choose folder
            </Button>
          </div>
        )}
        {status === 'previewing' && (
          <Button variant="secondary" onClick={cancelPreview}>
            Cancel
          </Button>
        )}
      </header>

      <p className="organize-safety-note">
        <ShieldCheck size={14} strokeWidth={2} aria-hidden="true" />
        Preview only — Nuvio never moves, renames, or deletes a file here.
      </p>

      {!rulesLoading && rules.length === 0 && (
        <Panel title="No rules yet">
          <EmptyState
            icon={FolderOpen}
            title="Add a rule before previewing"
            description="Organize previews your saved rules against a folder. Create a rule first."
            action={
              <Button variant="primary" onClick={() => onNavigate('rules')}>
                Go to Rules
              </Button>
            }
          />
        </Panel>
      )}

      {!rulesLoading && rules.length > 0 && status === 'idle' && (
        <Panel title="Nothing previewed yet">
          <EmptyState
            icon={FolderOpen}
            title="Choose a folder to preview"
            description="Nuvio will show what your rules would do, without changing anything."
            action={
              <Button variant="primary" onClick={() => void handleChooseFolder()}>
                Choose folder
              </Button>
            }
          />
        </Panel>
      )}

      {root && status !== 'idle' && <p className="organize-root">{root}</p>}

      {status === 'failed' && (
        <Panel title="Preview failed">
          <EmptyState
            icon={AlertCircle}
            title="Nuvio couldn't complete this preview"
            description={error ?? 'An unknown error occurred.'}
          />
        </Panel>
      )}

      {(status === 'previewing' || status === 'completed' || status === 'cancelled') && (
        <Panel
          title={
            status === 'previewing'
              ? 'Previewing…'
              : `${formatCount(entries.length)} files would move`
          }
          description={
            result?.truncated
              ? `Showing the first ${formatCount(entries.length)} matches — this is a lot to move at once.`
              : result
                ? `${formatBytes(result.totalSize)} total`
                : undefined
          }
        >
          {status === 'previewing' && (
            <div className="organize-progress">
              <ProgressBar />
              <div className="organize-progress-stats">
                <span>{formatCount(progress?.filesScanned ?? 0)} files scanned</span>
                <span>{formatCount(progress?.matchesFound ?? 0)} matches so far</span>
              </div>
            </div>
          )}

          {status === 'cancelled' && (
            <p className="organize-cancelled-notice">
              Preview cancelled — results below reflect what was found before you stopped it.
            </p>
          )}

          {entries.length > 0 ? (
            <PreviewEntryList entries={entries} />
          ) : (
            status !== 'previewing' && (
              <p className="organize-empty-note">No files in this folder match your rules.</p>
            )
          )}
        </Panel>
      )}
    </div>
  )
}
