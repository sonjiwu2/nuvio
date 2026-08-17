import { AlertCircle, FolderOpen, ListChecks, Plus } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../components/primitives/Button'
import { EmptyState } from '../components/primitives/EmptyState'
import { Panel } from '../components/primitives/Panel'
import { RuleList } from '../components/rules/RuleList'
import { useRules } from '../lib/useRules'
import { PickFolder } from '../../wailsjs/go/main/App'
import './Rules.css'

export function Rules() {
  const { rules, loading, error, addRule, deleteRule } = useRules()

  const [extension, setExtension] = useState('')
  const [destination, setDestination] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const canAdd = extension.trim().length > 0 && Boolean(destination) && !submitting

  async function handleChooseDestination() {
    const path = await PickFolder()
    if (path) setDestination(path)
  }

  async function handleAdd() {
    if (!canAdd || !destination) return
    setSubmitting(true)
    try {
      await addRule(extension.trim(), destination)
      setExtension('')
      setDestination(null)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="rules-screen">
      <header className="rules-header">
        <div>
          <h1 className="rules-title">Rules</h1>
          <p className="rules-subtitle">
            Define how Nuvio should organize your files by extension.
          </p>
        </div>
      </header>

      <Panel title="Add a rule" description="Files with this extension would move to this folder">
        <div className="rules-form">
          <input
            type="text"
            className="rules-form-input"
            placeholder="Extension, e.g. pdf"
            value={extension}
            onChange={(e) => setExtension(e.target.value)}
          />
          <Button variant="secondary" onClick={() => void handleChooseDestination()}>
            <FolderOpen size={15} strokeWidth={2} aria-hidden="true" />
            {destination ? 'Change destination' : 'Choose destination'}
          </Button>
          <Button variant="primary" disabled={!canAdd} onClick={() => void handleAdd()}>
            <Plus size={15} strokeWidth={2} aria-hidden="true" />
            Add rule
          </Button>
        </div>
        {destination && <p className="rules-form-destination">{destination}</p>}
      </Panel>

      {error && (
        <Panel title="Couldn't load rules">
          <EmptyState icon={AlertCircle} title="Something went wrong" description={error} />
        </Panel>
      )}

      {!error && !loading && (
        <Panel title="Your rules">
          {rules.length > 0 ? (
            <RuleList rules={rules} onDelete={(id) => void deleteRule(id)} />
          ) : (
            <EmptyState
              icon={ListChecks}
              title="No rules yet"
              description="Add a rule above, then preview it from Organize before anything is ever moved."
            />
          )}
        </Panel>
      )}
    </div>
  )
}
