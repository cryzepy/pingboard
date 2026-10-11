import { useState } from 'react'
import Sparkline from './Sparkline.jsx'
import TimeAgo from './TimeAgo.jsx'

const STATUS_LABEL = { up: 'ONLINE', down: 'OFFLINE', pending: 'MENUNGGU' }

export default function MonitorCard({ monitor: m, onDelete, onCheck }) {
  const [busy, setBusy] = useState('') // '' | 'check' | 'delete'
  const [error, setError] = useState('')

  async function run(kind, action) {
    setBusy(kind)
    setError('')
    try {
      await action(m.id)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy('')
    }
  }

  return (
    <article className={`card card--${m.status}`}>
      <div className="card-head">
        <div className="card-title">
          <h2 title={m.name}>{m.name}</h2>
          <a href={m.url} target="_blank" rel="noopener noreferrer" title={m.url}>
            {m.url}
          </a>
        </div>
        <span className={`badge badge--${m.status}`}>
          <span className="badge-dot" aria-hidden="true" />
          {STATUS_LABEL[m.status]}
        </span>
      </div>

      <Sparkline history={m.history} />

      <dl className="metrics">
        <div>
          <dt>Uptime</dt>
          <dd>{m.lastCheck ? `${m.uptimePct.toFixed(1)}%` : '–'}</dd>
        </div>
        <div>
          <dt>Latensi rata-rata</dt>
          <dd>{m.avgLatencyMs > 0 ? `${m.avgLatencyMs} ms` : '–'}</dd>
        </div>
        <div>
          <dt>Cek terakhir</dt>
          <dd>{m.lastCheck ? <TimeAgo iso={m.lastCheck.time} /> : '–'}</dd>
        </div>
      </dl>

      {m.status === 'down' && m.lastCheck?.error && (
        <p className="card-error">{m.lastCheck.error}</p>
      )}
      {error && (
        <p className="card-error" role="alert">
          {error}
        </p>
      )}

      <div className="card-actions">
        <span className="interval">tiap {formatInterval(m.intervalSec)}</span>
        <div className="card-buttons">
          <button
            className="btn"
            onClick={() => run('check', onCheck)}
            disabled={busy !== ''}
          >
            {busy === 'check' ? 'Mengecek…' : 'Cek sekarang'}
          </button>
          <button
            className="btn btn--danger"
            onClick={() => run('delete', onDelete)}
            disabled={busy !== ''}
            aria-label={`Hapus monitor ${m.name}`}
          >
            Hapus
          </button>
        </div>
      </div>
    </article>
  )
}

function formatInterval(sec) {
  if (sec < 60) return `${sec} dtk`
  if (sec % 60 === 0) return `${sec / 60} mnt`
  return `${sec} dtk`
}
