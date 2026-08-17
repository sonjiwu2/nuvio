import { File } from 'lucide-react'
import { formatBytes, truncatePath } from '../../lib/format'
import type { SearchMatch } from '../../lib/searchTypes'
import './MatchList.css'

interface MatchListProps {
  matches: SearchMatch[]
}

export function MatchList({ matches }: MatchListProps) {
  return (
    <ul className="match-list">
      {matches.map((match) => (
        <li key={match.path} className="match-row">
          <File size={16} strokeWidth={1.75} className="match-row-icon" aria-hidden="true" />
          <div className="match-row-info">
            <span className="match-row-name">{match.name}</span>
            <span className="match-row-path" title={match.path}>
              {truncatePath(match.path, 72)}
            </span>
          </div>
          <span className="match-row-size">{formatBytes(match.size)}</span>
        </li>
      ))}
    </ul>
  )
}
