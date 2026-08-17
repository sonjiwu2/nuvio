import { ArrowRight, Trash2 } from 'lucide-react'
import { truncatePath } from '../../lib/format'
import type { Rule } from '../../lib/ruleTypes'
import './RuleList.css'

interface RuleListProps {
  rules: Rule[]
  onDelete: (id: string) => void
}

export function RuleList({ rules, onDelete }: RuleListProps) {
  return (
    <ul className="rule-list">
      {rules.map((rule) => (
        <li key={rule.id} className="rule-row">
          <span className="rule-row-extension">.{rule.extension}</span>
          <ArrowRight size={14} strokeWidth={2} className="rule-row-arrow" aria-hidden="true" />
          <span className="rule-row-destination" title={rule.destinationFolder}>
            {truncatePath(rule.destinationFolder, 56)}
          </span>
          <button
            type="button"
            className="rule-row-delete"
            aria-label={`Delete rule for .${rule.extension}`}
            onClick={() => onDelete(rule.id)}
          >
            <Trash2 size={15} strokeWidth={2} aria-hidden="true" />
          </button>
        </li>
      ))}
    </ul>
  )
}
