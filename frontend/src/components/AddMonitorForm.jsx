import { useState } from 'react'

const INTERVALS = [
  { value: 10, label: '10 detik' },
  { value: 30, label: '30 detik' },
  { value: 60, label: '1 menit' },
  { value: 300, label: '5 menit' },
]

export default function AddMonitorForm({ onCreate }) {
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [intervalSec, setIntervalSec] = useState(30)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await onCreate({ name, url, intervalSec })
      setName('')
      setUrl('')
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form className="form" onSubmit={handleSubmit}>
      <div className="field field--grow">
        <label htmlFor="url">URL</label>
        <input
          id="url"
          type="url"
          required
          placeholder="https://contoh.com"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
        />
      </div>
      <div className="field">
        <label htmlFor="name">Nama (opsional)</label>
        <input
          id="name"
          type="text"
          maxLength={60}
          placeholder="Situs saya"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>
      <div className="field">
        <label htmlFor="interval">Cek setiap</label>
        <select
          id="interval"
          value={intervalSec}
          onChange={(e) => setIntervalSec(Number(e.target.value))}
        >
          {INTERVALS.map((i) => (
            <option key={i.value} value={i.value}>
              {i.label}
            </option>
          ))}
        </select>
      </div>
      <button className="btn btn--primary" type="submit" disabled={busy}>
        {busy ? 'Menambah…' : 'Tambah monitor'}
      </button>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
    </form>
  )
}
