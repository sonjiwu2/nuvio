import { ArrowRight, File } from 'lucide-react'
import { formatBytes, truncatePath } from '../../lib/format'
import type { PreviewEntry } from '../../lib/ruleTypes'
import './PreviewEntryList.css'

interface PreviewEntryListProps {
  entries: PreviewEntry[]
}

export function PreviewEntryList({ entries }: PreviewEntryListProps) {
  return (
    <ul className="preview-entry-list">
      {entries.map((entry) => (
        <li key={entry.sourcePath} className="preview-entry-row">
          <File size={16} strokeWidth={1.75} className="preview-entry-icon" aria-hidden="true" />
          <div className="preview-entry-paths">
            <span className="preview-entry-name">{entry.name}</span>
            <span className="preview-entry-move">
              {truncatePath(entry.sourcePath, 40)}
              <ArrowRight size={12} strokeWidth={2} aria-hidden="true" />
              {truncatePath(entry.destinationPath, 40)}
            </span>
          </div>
          <span className="preview-entry-size">{formatBytes(entry.size)}</span>
        </li>
      ))}
    </ul>
  )
}
