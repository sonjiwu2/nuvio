import type { PropsWithChildren } from 'react'
import type { ScreenKey } from '../../lib/navigation'
import { Sidebar } from './Sidebar'
import './AppShell.css'

interface AppShellProps extends PropsWithChildren {
  active: ScreenKey
  onNavigate: (key: ScreenKey) => void
}

export function AppShell({ active, onNavigate, children }: AppShellProps) {
  return (
    <div className="app-shell">
      <Sidebar active={active} onNavigate={onNavigate} />
      <main className="app-shell-content">{children}</main>
    </div>
  )
}
