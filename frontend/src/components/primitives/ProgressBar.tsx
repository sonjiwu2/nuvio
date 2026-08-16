import './ProgressBar.css'

interface ProgressBarProps {
  /** Omit to render an indeterminate (unknown-percentage) bar. */
  percent?: number
}

export function ProgressBar({ percent }: ProgressBarProps) {
  const indeterminate = percent === undefined
  return (
    <div
      className={`progress-bar${indeterminate ? ' progress-bar--indeterminate' : ''}`}
      role="progressbar"
      aria-valuenow={indeterminate ? undefined : Math.round(percent)}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <div
        className="progress-bar-fill"
        style={indeterminate ? undefined : { width: `${percent}%` }}
      />
    </div>
  )
}
