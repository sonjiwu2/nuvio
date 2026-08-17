import { useCallback, useEffect, useState } from 'react'
import { AddRule, DeleteRule, ListRules } from '../../wailsjs/go/main/App'
import type { Rule } from './ruleTypes'

/** Loads and mutates the saved Organize rules (extension -> destination folder). */
export function useRules() {
  const [rules, setRules] = useState<Rule[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Deliberately does not set loading=true itself: `loading` already
  // starts true, and refresh() is also called after add/delete where a
  // full loading state would just cause an unwanted flicker. All setState
  // calls below happen only in the async continuation, after `await
  // ListRules()` — never synchronously — so this is the standard,
  // React-docs-endorsed "fetch on mount" pattern, not an update loop.
  const refresh = useCallback(async () => {
    try {
      const list = await ListRules()
      setRules(list ?? [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    // react-hooks/set-state-in-effect flags this without control-flow
    // analysis: it can't see that refresh()'s setState calls only happen
    // after an await, never synchronously in this effect body. Loading
    // data on mount via a stable, useCallback-memoized async function is
    // the pattern React's own docs recommend when the fetch can't be
    // expressed any other way (see useSearch/useScan's EventsOn-based
    // effects, which don't need this because they only subscribe).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void refresh()
  }, [refresh])

  const addRule = useCallback(
    async (extension: string, destinationFolder: string) => {
      await AddRule(extension, destinationFolder)
      await refresh()
    },
    [refresh],
  )

  const deleteRule = useCallback(
    async (id: string) => {
      await DeleteRule(id)
      await refresh()
    },
    [refresh],
  )

  return { rules, loading, error, addRule, deleteRule }
}
