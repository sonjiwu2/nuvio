import { Copy } from 'lucide-react'
import { formatBytes, formatCount, truncatePath } from '../../lib/format'
import type { DuplicateGroup } from '../../lib/duplicateTypes'
import './DuplicateGroupList.css'

interface DuplicateGroupListProps {
  groups: DuplicateGroup[]
}

function reclaimable(group: DuplicateGroup): number {
  return group.files.length > 1 ? group.size * (group.files.length - 1) : 0
}

export function DuplicateGroupList({ groups }: DuplicateGroupListProps) {
  return (
    <ul className="duplicate-group-list">
      {groups.map((group) => (
        <li key={group.hash} className="duplicate-group">
          <details>
            <summary className="duplicate-group-summary">
              <Copy size={15} strokeWidth={2} aria-hidden="true" />
              <span className="duplicate-group-count">
                {formatCount(group.files.length)} copies
              </span>
              <span className="duplicate-group-size">{formatBytes(group.size)} each</span>
              <span className="duplicate-group-reclaim">
                {formatBytes(reclaimable(group))} reclaimable
              </span>
            </summary>
            <ul className="duplicate-group-files">
              {group.files.map((file) => (
                <li key={file.path} className="duplicate-group-file" title={file.path}>
                  {truncatePath(file.path, 90)}
                </li>
              ))}
            </ul>
          </details>
        </li>
      ))}
    </ul>
  )
}
