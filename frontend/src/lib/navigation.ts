import type { LucideIcon } from 'lucide-react'
import {
  Activity,
  Copy,
  FolderKanban,
  HardDrive,
  Home,
  ListChecks,
  Search,
  Settings,
  Sparkles,
} from 'lucide-react'

export type ScreenKey =
  | 'home'
  | 'organize'
  | 'rules'
  | 'search'
  | 'storage'
  | 'duplicates'
  | 'cleanup'
  | 'activity'
  | 'settings'

export interface NavItem {
  key: ScreenKey
  label: string
  icon: LucideIcon
  /** Screens with no backend behind them yet show a honest placeholder. */
  implemented: boolean
}

export const NAV_ITEMS: NavItem[] = [
  { key: 'home', label: 'Home', icon: Home, implemented: true },
  { key: 'organize', label: 'Organize', icon: FolderKanban, implemented: true },
  { key: 'rules', label: 'Rules', icon: ListChecks, implemented: true },
  { key: 'search', label: 'Search', icon: Search, implemented: true },
  { key: 'storage', label: 'Storage', icon: HardDrive, implemented: true },
  { key: 'duplicates', label: 'Duplicates', icon: Copy, implemented: false },
  { key: 'cleanup', label: 'Cleanup', icon: Sparkles, implemented: false },
  { key: 'activity', label: 'Activity', icon: Activity, implemented: false },
  { key: 'settings', label: 'Settings', icon: Settings, implemented: false },
]
