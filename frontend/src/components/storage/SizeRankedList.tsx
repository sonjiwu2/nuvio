import { formatBytes, truncatePath } from '../../lib/format'
import './SizeRankedList.css'

interface SizeRankedItem {
  key: string
  primaryLabel: string
  secondaryLabel: string
  size: number
}

interface SizeRankedListProps {
  items: SizeRankedItem[]
  totalSize: number
}

/** A ranked list of paths with a size bar, used for both largest folders and largest files. */
export function SizeRankedList({ items, totalSize }: SizeRankedListProps) {
  return (
    <ul className="size-ranked-list">
      {items.map((item) => {
        const percent = totalSize > 0 ? Math.max((item.size / totalSize) * 100, 1) : 0
        return (
          <li key={item.key} className="size-ranked-row">
            <div className="size-ranked-row-info">
              <span className="size-ranked-row-name" title={item.secondaryLabel}>
                {item.primaryLabel}
              </span>
              <span className="size-ranked-row-path">{truncatePath(item.secondaryLabel, 48)}</span>
            </div>
            <div className="size-ranked-row-bar">
              <div className="size-ranked-row-bar-fill" style={{ width: `${percent}%` }} />
            </div>
            <span className="size-ranked-row-size">{formatBytes(item.size)}</span>
          </li>
        )
      })}
    </ul>
  )
}
