import { LifeBuoy } from 'lucide-react'
import { useState } from 'react'
import { AppShell } from './components/layout/AppShell'
import { NAV_ITEMS, type ScreenKey } from './lib/navigation'
import { ComingSoon } from './screens/ComingSoon'
import { Duplicates } from './screens/Duplicates'
import { Home } from './screens/Home'
import { Organize } from './screens/Organize'
import { Rules } from './screens/Rules'
import { Search } from './screens/Search'
import { Storage } from './screens/Storage'

function App() {
  const [active, setActive] = useState<ScreenKey>('home')

  return (
    <AppShell active={active} onNavigate={setActive}>
      {renderScreen(active, setActive)}
    </AppShell>
  )
}

function renderScreen(active: ScreenKey, onNavigate: (key: ScreenKey) => void) {
  if (active === 'home') return <Home onNavigate={onNavigate} />
  if (active === 'storage') return <Storage />
  if (active === 'search') return <Search />
  if (active === 'organize') return <Organize onNavigate={onNavigate} />
  if (active === 'rules') return <Rules />
  if (active === 'duplicates') return <Duplicates />

  const item = NAV_ITEMS.find((entry) => entry.key === active)
  return <ComingSoon title={item?.label ?? 'Coming soon'} icon={item?.icon ?? LifeBuoy} />
}

export default App
