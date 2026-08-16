import { ShieldCheck } from 'lucide-react'
import { NAV_ITEMS, type ScreenKey } from '../../lib/navigation'
import './Sidebar.css'

interface SidebarProps {
  active: ScreenKey
  onNavigate: (key: ScreenKey) => void
}

export function Sidebar({ active, onNavigate }: SidebarProps) {
  return (
    <nav className="sidebar" aria-label="Primary">
      <div className="sidebar-brand">
        <div className="sidebar-brand-mark">N</div>
        <span className="sidebar-brand-name">Nuvio</span>
      </div>

      <ul className="sidebar-nav">
        {NAV_ITEMS.map((item) => {
          const Icon = item.icon
          const isActive = item.key === active
          return (
            <li key={item.key}>
              <button
                type="button"
                className={`sidebar-nav-item${isActive ? ' is-active' : ''}`}
                aria-current={isActive ? 'page' : undefined}
                onClick={() => onNavigate(item.key)}
              >
                <Icon size={17} strokeWidth={2} aria-hidden="true" />
                <span>{item.label}</span>
                {!item.implemented && <span className="sidebar-nav-badge">Soon</span>}
              </button>
            </li>
          )
        })}
      </ul>

      <div className="sidebar-footer">
        <ShieldCheck size={18} strokeWidth={2} aria-hidden="true" />
        <div>
          <div className="sidebar-footer-title">Local-first.</div>
          <div className="sidebar-footer-subtitle">Your files stay on your device.</div>
        </div>
      </div>
    </nav>
  )
}
