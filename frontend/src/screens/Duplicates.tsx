import { AlertCircle, Copy, FolderOpen, HardDrive, ShieldCheck } from 'lucide-react'
import { Button } from '../components/primitives/Button'
import { DuplicateGroupList } from '../components/duplicates/DuplicateGroupList'
import { EmptyState } from '../components/primitives/EmptyState'
import { MetricCard } from '../components/primitives/MetricCard'
import { Panel } from '../components/primitives/Panel'
import { ProgressBar } from '../components/primitives/ProgressBar'
import { formatBytes, formatCount } from '../lib/format'
import { useDuplicates } from '../lib/useDuplicates'
import './Duplicates.css'

const PHASE_LABEL: Record<string, string> = {
  scanning: 'Scanning files…',
  hashing: 'Comparing content…',
}

export function Duplicates() {
  const { state, pickFolder, startScan, cancelScan } = useDuplicates()
  const { status, root, progress, result, error } = state

  async function handleChooseFolder() {
    const path = await pickFolder()
    if (path) await startScan(path)
  }

  async function handleRescan() {
    if (root) await startScan(root)
  }

  return (
    <div className="duplicates-screen">
      <header className="duplicates-header">
        <div>
          <h1 className="duplicates-title">Duplicates</h1>
          <p className="duplicates-subtitle">Find files with identical content.</p>
        </div>
        <div className="duplicates-actions">
          {status === 'scanning' ? (
            <Button variant="secondary" onClick={cancelScan}>
              Cancel
            </Button>
          ) : (
            <>
              {root && (
                <Button variant="secondary" onClick={() => void handleRescan()}>
                  Rescan
                </Button>
              )}
              <Button variant="primary" onClick={() => void handleChooseFolder()}>
                <FolderOpen size={15} strokeWidth={2} aria-hidden="true" />
                Choose folder
              </Button>
            </>
          )}
        </div>
      </header>

      <p className="duplicates-safety-note">
        <ShieldCheck size={14} strokeWidth={2} aria-hidden="true" />
        Detection only — Nuvio never deletes a file here.
      </p>

      {root && <p className="duplicates-root">{root}</p>}

      {status === 'idle' && (
        <Panel title="Nothing scanned yet">
          <EmptyState
            icon={Copy}
            title="Choose a folder to find duplicates"
            description="Nuvio compares file content, not just names or sizes."
            action={
              <Button variant="primary" onClick={() => void handleChooseFolder()}>
                Choose folder
              </Button>
            }
          />
        </Panel>
      )}

      {status === 'scanning' && (
        <Panel
          title={PHASE_LABEL[progress?.phase ?? 'scanning'] ?? 'Scanning files…'}
          description={root ?? undefined}
        >
          <div className="duplicates-progress">
            <ProgressBar />
            <div className="duplicates-progress-stats">
              <span>{formatCount(progress?.filesScanned ?? 0)} files found</span>
              <span>{formatCount(progress?.filesHashed ?? 0)} files compared</span>
            </div>
          </div>
        </Panel>
      )}

      {status === 'failed' && (
        <Panel title="Scan failed">
          <EmptyState
            icon={AlertCircle}
            title="Nuvio couldn't complete this scan"
            description={error ?? 'An unknown error occurred.'}
          />
        </Panel>
      )}

      {(status === 'completed' || status === 'cancelled') && result && (
        <div className="duplicates-results">
          {status === 'cancelled' && (
            <p className="duplicates-cancelled-notice">
              Scan cancelled — results below reflect what was found before you stopped it.
            </p>
          )}

          <div className="duplicates-metrics">
            <MetricCard
              label="Reclaimable space"
              icon={HardDrive}
              iconTone="violet"
              value={formatBytes(result.totalReclaimable)}
              hint="if you keep one copy of each"
            />
            <MetricCard
              label="Duplicate groups"
              icon={Copy}
              iconTone="blue"
              value={formatCount(result.groups.length)}
              hint={result.truncated ? 'showing the largest' : 'found'}
            />
            <MetricCard
              label="Files compared"
              icon={HardDrive}
              iconTone="teal"
              value={formatCount(result.filesHashed)}
              hint={`of ${formatCount(result.filesScanned)} scanned`}
            />
          </div>

          <Panel
            title="Duplicate groups"
            description={
              result.truncated
                ? 'Showing the groups with the most reclaimable space — there were more.'
                : 'Ranked by reclaimable space'
            }
          >
            {result.groups.length > 0 ? (
              <DuplicateGroupList groups={result.groups} />
            ) : (
              <p className="duplicates-empty-note">No duplicate files found.</p>
            )}
          </Panel>
        </div>
      )}
    </div>
  )
}
