import type { LucideIcon } from 'lucide-react'
import type { PropsWithChildren, ReactNode } from 'react'
import './EmptyState.css'

interface EmptyStateProps extends PropsWithChildren {
  icon: LucideIcon
  title: string
  description: string
  action?: ReactNode
}

export function EmptyState({ icon: Icon, title, description, action, children }: EmptyStateProps) {
  return (
    <div className="empty-state">
      <div className="empty-state-icon">
        <Icon size={22} strokeWidth={1.75} aria-hidden="true" />
      </div>
      <div className="empty-state-title">{title}</div>
      <p className="empty-state-description">{description}</p>
      {action}
      {children}
    </div>
  )
}
