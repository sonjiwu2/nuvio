import { useEffect, useRef, useState } from 'react'
import { formatBytes, truncatePath } from '../../lib/format'
import { computeTreemap, type TreemapItem } from '../../lib/treemap'
import './Treemap.css'

const PALETTE = ['blue', 'teal', 'violet', 'amber', 'rose', 'slate'] as const

interface TreemapProps {
  items: TreemapItem[]
}

export function Treemap({ items }: TreemapProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [size, setSize] = useState({ width: 0, height: 0 })

  useEffect(() => {
    const el = containerRef.current
    if (!el) return

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (!entry) return
      const { width, height } = entry.contentRect
      setSize({ width, height })
    })
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const rects = computeTreemap(items, size.width, size.height)

  return (
    <div ref={containerRef} className="treemap">
      {rects.map((rect, index) => (
        <div
          key={rect.key}
          className={`treemap-cell treemap-cell--${PALETTE[index % PALETTE.length]}`}
          style={{
            left: rect.x,
            top: rect.y,
            width: Math.max(rect.width - 2, 0),
            height: Math.max(rect.height - 2, 0),
          }}
          title={`${rect.label} — ${formatBytes(rect.value)}`}
        >
          {rect.width > 60 && rect.height > 34 && (
            <>
              <span className="treemap-cell-label">{truncatePath(rect.label, 24)}</span>
              <span className="treemap-cell-size">{formatBytes(rect.value)}</span>
            </>
          )}
        </div>
      ))}
    </div>
  )
}
