import { AlertCircle, FolderOpen, Search as SearchIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../components/primitives/Button'
import { EmptyState } from '../components/primitives/EmptyState'
import { Panel } from '../components/primitives/Panel'
import { ProgressBar } from '../components/primitives/ProgressBar'
import { MatchList } from '../components/search/MatchList'
import { formatCount } from '../lib/format'
import { useSearch } from '../lib/useSearch'
import './Search.css'

export function Search() {
  const { state, pickFolder, startSearch, cancelSearch } = useSearch()
  const { status, matches, progress, result, error } = state

  const [folder, setFolder] = useState<string | null>(null)
  const [query, setQuery] = useState('')

  const canSearch = Boolean(folder) && query.trim().length > 0 && status !== 'searching'

  async function handleChooseFolder() {
    const path = await pickFolder()
    if (path) setFolder(path)
  }

  async function handleSearch() {
    if (!folder || !query.trim()) return
    await startSearch(folder, query.trim())
  }

  return (
    <div className="search-screen">
      <header className="search-header">
        <div>
          <h1 className="search-title">Search</h1>
          <p className="search-subtitle">Find files by name across a folder.</p>
        </div>
      </header>

      <div className="search-controls">
        <Button variant="secondary" onClick={() => void handleChooseFolder()}>
          <FolderOpen size={15} strokeWidth={2} aria-hidden="true" />
          {folder ? 'Change folder' : 'Choose folder'}
        </Button>
        <input
          type="text"
          className="search-input"
          placeholder="File name contains…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && canSearch) void handleSearch()
          }}
          disabled={!folder}
        />
        {status === 'searching' ? (
          <Button variant="secondary" onClick={cancelSearch}>
            Cancel
          </Button>
        ) : (
          <Button variant="primary" disabled={!canSearch} onClick={() => void handleSearch()}>
            <SearchIcon size={15} strokeWidth={2} aria-hidden="true" />
            Search
          </Button>
        )}
      </div>

      {folder && <p className="search-root">{folder}</p>}

      {status === 'idle' && (
        <Panel title="Nothing searched yet">
          <EmptyState
            icon={SearchIcon}
            title="Choose a folder and type a search term"
            description="Nuvio will look through file names inside the folder you pick."
          />
        </Panel>
      )}

      {status === 'failed' && (
        <Panel title="Search failed">
          <EmptyState
            icon={AlertCircle}
            title="Nuvio couldn't complete this search"
            description={error ?? 'An unknown error occurred.'}
          />
        </Panel>
      )}

      {(status === 'searching' || status === 'completed' || status === 'cancelled') && (
        <Panel
          title={status === 'searching' ? 'Searching…' : `${formatCount(matches.length)} results`}
          description={
            result?.truncated
              ? `Showing the first ${formatCount(matches.length)} matches — narrow your search for more precise results.`
              : undefined
          }
        >
          {status === 'searching' && (
            <div className="search-progress">
              <ProgressBar />
              <div className="search-progress-stats">
                <span>{formatCount(progress?.filesScanned ?? 0)} files scanned</span>
                <span>{formatCount(progress?.matchesFound ?? 0)} matches so far</span>
              </div>
            </div>
          )}

          {status === 'cancelled' && (
            <p className="search-cancelled-notice">
              Search cancelled — results below reflect what was found before you stopped it.
            </p>
          )}

          {matches.length > 0 ? (
            <MatchList matches={matches} />
          ) : (
            status !== 'searching' && (
              <p className="search-empty-note">No files matched your search.</p>
            )
          )}
        </Panel>
      )}
    </div>
  )
}
