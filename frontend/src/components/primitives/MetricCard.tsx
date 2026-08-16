import type { LucideIcon } from 'lucide-react'
import './MetricCard.css'

interface MetricCardProps {
  label: string
  icon: LucideIcon
  iconTone: 'blue' | 'teal' | 'violet' | 'amber'
  /** Pass undefined when the feature behind this metric hasn't run yet — never a fabricated number. */
  value?: string
  hint: string
}

export function MetricCard({ label, icon: Icon, iconTone, value, hint }: MetricCardProps) {
  return (
    <div className="metric-card">
      <div className={`metric-card-icon metric-card-icon--${iconTone}`}>
        <Icon size={18} strokeWidth={2} aria-hidden="true" />
      </div>
      <div className="metric-card-body">
        <div className="metric-card-label">{label}</div>
        <div className={`metric-card-value${value ? '' : ' is-empty'}`}>{value ?? '—'}</div>
        <div className="metric-card-hint">{hint}</div>
      </div>
    </div>
  )
}
