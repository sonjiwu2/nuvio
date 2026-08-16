import type { PropsWithChildren, ReactNode } from 'react'
import './Panel.css'

interface PanelProps extends PropsWithChildren {
  title: string
  description?: string
  action?: ReactNode
  className?: string
}

export function Panel({ title, description, action, className, children }: PanelProps) {
  return (
    <section className={`panel${className ? ` ${className}` : ''}`}>
      <header className="panel-header">
        <div>
          <h2 className="panel-title">{title}</h2>
          {description && <p className="panel-description">{description}</p>}
        </div>
        {action}
      </header>
      <div className="panel-body">{children}</div>
    </section>
  )
}
