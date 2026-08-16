import { Activity, Copy, FolderTree, HardDrive, Search, Sparkles, Wand2 } from 'lucide-react'
import { Button } from '../components/primitives/Button'
import { EmptyState } from '../components/primitives/EmptyState'
import { MetricCard } from '../components/primitives/MetricCard'
import { Panel } from '../components/primitives/Panel'
import type { ScreenKey } from '../lib/navigation'
import './Home.css'

function greeting(): string {
  const hour = new Date().getHours()
  if (hour < 12) return 'Good morning'
  if (hour < 18) return 'Good afternoon'
  return 'Good evening'
}

interface HomeProps {
  onNavigate: (key: ScreenKey) => void
}

export function Home({ onNavigate }: HomeProps) {
  return (
    <div className="home">
      <header className="home-header">
        <div>
          <h1 className="home-title">{greeting()}</h1>
          <p className="home-subtitle">Choose a folder to see what is taking up space.</p>
        </div>
        <div className="home-actions">
          <div className="home-search">
            <Search size={16} strokeWidth={2} aria-hidden="true" />
            <input type="text" placeholder="Search files, folders, or tags..." disabled />
          </div>
          <Button variant="primary" onClick={() => onNavigate('storage')}>
            Scan a folder
          </Button>
        </div>
      </header>

      <div className="home-metrics">
        <MetricCard
          label="Files organized"
          icon={FolderTree}
          iconTone="blue"
          hint="Available once Rules is set up"
        />
        <MetricCard
          label="Potential cleanup"
          icon={Wand2}
          iconTone="teal"
          hint="Available once Cleanup is set up"
        />
        <MetricCard label="Duplicate files" icon={Copy} iconTone="violet" hint="Not scanned yet" />
        <MetricCard
          label="Watched folders"
          icon={HardDrive}
          iconTone="amber"
          hint="No folders watched yet"
        />
      </div>

      <div className="home-grid">
        <Panel title="Storage Overview" description="Everything on your device">
          <EmptyState
            icon={HardDrive}
            title="Nothing scanned yet"
            description="Choose a folder to see what is taking up space on your device."
            action={
              <Button variant="secondary" onClick={() => onNavigate('storage')}>
                Go to Storage
              </Button>
            }
          />
        </Panel>

        <Panel title="Suggestions" description="Smart ideas to keep things organized">
          <EmptyState
            icon={Sparkles}
            title="No suggestions yet"
            description="Nuvio will suggest ways to organize your files once Rules is available."
          />
        </Panel>

        <Panel title="Developer Cleanup" description="Reclaim space from dev artifacts">
          <EmptyState
            icon={Copy}
            title="Coming soon"
            description="Detecting node_modules, build caches, and other reclaimable project artifacts is planned for a future update."
          />
        </Panel>
      </div>

      <Panel
        title="Recent Activity"
        description="What Nuvio has done recently"
        className="home-activity"
      >
        <EmptyState
          icon={Activity}
          title="No activity yet"
          description="Actions Nuvio takes on your files — organizing, cleanup, undo — will show up here."
        />
      </Panel>
    </div>
  )
}
