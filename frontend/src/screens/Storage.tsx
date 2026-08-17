import {
  AlertCircle,
  Clock,
  File,
  FolderOpen,
  FolderTree,
  HardDrive,
  RotateCcw,
} from 'lucide-react'
import { Button } from '../components/primitives/Button'
import { EmptyState } from '../components/primitives/EmptyState'
import { MetricCard } from '../components/primitives/MetricCard'
import { Panel } from '../components/primitives/Panel'
import { ProgressBar } from '../components/primitives/ProgressBar'
import { ScanIssuesNotice } from '../components/storage/ScanIssuesNotice'
import { SizeRankedList } from '../components/storage/SizeRankedList'
import { Treemap } from '../components/storage/Treemap'
import { formatBytes, formatCount, formatDuration, truncatePath } from '../lib/format'
import { useScan } from '../lib/useScan'
import './Storage.css'

export function Storage() {
  const { state, pickFolder, startScan, cancelScan } = useScan()
  const { status, root, progress, result, error } = state

  async function handleChooseFolder() {
    const path = await pickFolder()
    if (path) {
      await startScan(path)
    }
  }

  async function handleRescan() {
    if (root) await startScan(root)
  }

  return (
    <div className="storage-screen">
      <header className="storage-header">
        <div>
          <h1 className="storage-title">Storage</h1>
          <p className="storage-subtitle">Understand what is taking up space on your device.</p>
        </div>
        <div className="storage-actions">
          {status === 'scanning' ? (
            <Button variant="secondary" onClick={cancelScan}>
              Cancel
            </Button>
          ) : (
            <>
              {root && (
                <Button variant="secondary" onClick={() => void handleRescan()}>
                  <RotateCcw size={15} strokeWidth={2} aria-hidden="true" />
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

      {root && <p className="storage-root">{root}</p>}

      {status === 'idle' && (
        <Panel title="Nothing scanned yet">
          <EmptyState
            icon={HardDrive}
            title="Choose a folder to get started"
            description="Nuvio will scan it and show you the largest folders and files."
            action={
              <Button variant="primary" onClick={() => void handleChooseFolder()}>
                Choose folder
              </Button>
            }
          />
        </Panel>
      )}

      {status === 'scanning' && (
        <Panel title="Scanning…" description={root ?? undefined}>
          <div className="storage-progress">
            <ProgressBar />
            <div className="storage-progress-stats">
              <span>{formatCount(progress?.filesScanned ?? 0)} files</span>
              <span>{formatCount(progress?.dirsScanned ?? 0)} folders</span>
              <span>{formatBytes(progress?.bytesScanned ?? 0)} processed</span>
            </div>
            {progress?.currentPath && (
              <p className="storage-progress-path">{truncatePath(progress.currentPath, 90)}</p>
            )}
          </div>
        </Panel>
      )}

      {status === 'failed' && (
        <Panel title="Scan failed">
          <EmptyState
            icon={AlertCircle}
            title="Nuvio couldn't complete this scan"
            description={error ?? 'An unknown error occurred.'}
            action={
              <Button variant="secondary" onClick={() => void handleChooseFolder()}>
                Choose a different folder
              </Button>
            }
          />
        </Panel>
      )}

      {(status === 'completed' || status === 'cancelled') && result && (
        <div className="storage-results">
          {status === 'cancelled' && (
            <p className="storage-cancelled-notice">
              Scan cancelled — totals below reflect what was scanned before you stopped it.
            </p>
          )}

          <div className="storage-metrics">
            <MetricCard
              label="Total size"
              icon={HardDrive}
              iconTone="blue"
              value={formatBytes(result.totalSize)}
              hint={truncatePath(result.root, 40)}
            />
            <MetricCard
              label="Files"
              icon={File}
              iconTone="teal"
              value={formatCount(result.totalFiles)}
              hint="scanned"
            />
            <MetricCard
              label="Folders"
              icon={FolderTree}
              iconTone="violet"
              value={formatCount(result.totalDirs)}
              hint="scanned"
            />
            <MetricCard
              label="Duration"
              icon={Clock}
              iconTone="amber"
              value={formatDuration(result.durationNs)}
              hint="scan time"
            />
          </div>

          {result.issues.length > 0 && <ScanIssuesNotice issues={result.issues} />}

          <Panel title="Storage Overview" description="What's inside the folder you scanned">
            {result.rootChildren.length > 0 ? (
              <Treemap
                items={result.rootChildren.map((f) => ({
                  key: f.path,
                  label: f.name,
                  value: f.size,
                }))}
              />
            ) : (
              <p className="storage-empty-note">
                Nothing to visualize — the scanned folder is empty.
              </p>
            )}
          </Panel>

          <div className="storage-grid">
            <Panel
              title="Largest folders"
              description="Ranked by total size, including everything inside them"
            >
              {result.topFolders.length > 0 ? (
                <SizeRankedList
                  totalSize={result.totalSize}
                  items={result.topFolders.map((f) => ({
                    key: f.path,
                    primaryLabel: f.name,
                    secondaryLabel: f.path,
                    size: f.size,
                  }))}
                />
              ) : (
                <p className="storage-empty-note">No folders found.</p>
              )}
            </Panel>

            <Panel title="Largest files" description="Individual files taking up the most space">
              {result.topFiles.length > 0 ? (
                <SizeRankedList
                  totalSize={result.totalSize}
                  items={result.topFiles.map((f) => ({
                    key: f.path,
                    primaryLabel: f.name,
                    secondaryLabel: f.path,
                    size: f.size,
                  }))}
                />
              ) : (
                <p className="storage-empty-note">No files found.</p>
              )}
            </Panel>
          </div>
        </div>
      )}
    </div>
  )
}
