import type { LucideIcon } from 'lucide-react'
import { EmptyState } from '../components/primitives/EmptyState'
import './ComingSoon.css'

interface ComingSoonProps {
  title: string
  icon: LucideIcon
}

export function ComingSoon({ title, icon }: ComingSoonProps) {
  return (
    <div className="coming-soon">
      <h1 className="coming-soon-title">{title}</h1>
      <div className="coming-soon-panel">
        <EmptyState
          icon={icon}
          title="Not built yet"
          description={`${title} is on the Nuvio roadmap but isn't implemented in this build.`}
        />
      </div>
    </div>
  )
}
