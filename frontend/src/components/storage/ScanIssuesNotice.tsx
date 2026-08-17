import { AlertTriangle } from 'lucide-react'
import type { ScanIssue } from '../../lib/scanTypes'
import './ScanIssuesNotice.css'

interface ScanIssuesNoticeProps {
  issues: ScanIssue[]
}

export function ScanIssuesNotice({ issues }: ScanIssuesNoticeProps) {
  if (issues.length === 0) return null

  return (
    <details className="scan-issues">
      <summary className="scan-issues-summary">
        <AlertTriangle size={15} strokeWidth={2} aria-hidden="true" />
        Completed with {issues.length} {issues.length === 1 ? 'issue' : 'issues'}
      </summary>
      <ul className="scan-issues-list">
        {issues.map((issue) => (
          <li key={issue.path}>
            <span className="scan-issues-path">{issue.path}</span>
            <span className="scan-issues-error">{issue.error}</span>
          </li>
        ))}
      </ul>
    </details>
  )
}
