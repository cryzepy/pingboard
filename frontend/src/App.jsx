import { useCallback, useEffect, useMemo, useState } from 'react'
import { checkNow, createMonitor, deleteMonitor, listMonitors } from './api.js'
import AddMonitorForm from './components/AddMonitorForm.jsx'
import MonitorCard from './components/MonitorCard.jsx'
import Summary from './components/Summary.jsx'

const POLL_MS = 5000

export default function App() {
  const [monitors, setMonitors] = useState(null) // null = belum dimuat
  const [connectionError, setConnectionError] = useState(false)

  const refresh = useCallback(async (signal) => {
    try {
      const data = await listMonitors(signal)
      setMonitors(data)
      setConnectionError(false)
    } catch (err) {
      if (err.name !== 'AbortError') setConnectionError(true)
    }
  }, [])

  // Polling berkala; dibersihkan saat komponen dilepas.
  useEffect(() => {
    const controller = new AbortController()
    refresh(controller.signal)
    const timer = setInterval(() => refresh(controller.signal), POLL_MS)
    return () => {
      controller.abort()
      clearInterval(timer)
    }
  }, [refresh])

  const handleCreate = useCallback(
    async (input) => {
      const created = await createMonitor(input)
      setMonitors((prev) => [...(prev ?? []), created])
      // backend langsung mengecek; ambil hasilnya sebentar lagi
      setTimeout(() => refresh(), 1200)
    },
    [refresh],
  )

  const handleDelete = useCallback(async (id) => {
    await deleteMonitor(id)
    setMonitors((prev) => (prev ?? []).filter((m) => m.id !== id))
  }, [])

  const handleCheck = useCallback(async (id) => {
    const updated = await checkNow(id)
    setMonitors((prev) => (prev ?? []).map((m) => (m.id === id ? updated : m)))
  }, [])

  const stats = useMemo(() => {
    const list = monitors ?? []
    const checked = list.filter((m) => m.status !== 'pending')
    return {
      total: list.length,
      up: list.filter((m) => m.status === 'up').length,
      down: list.filter((m) => m.status === 'down').length,
      avgUptime: checked.length
        ? checked.reduce((sum, m) => sum + m.uptimePct, 0) / checked.length
        : null,
    }
  }, [monitors])

  return (
    <div className="app">
      <header className="header">
        <div className="brand">
          <svg className="brand-icon" viewBox="0 0 32 32" aria-hidden="true">
            <path
              d="M4 17h7l3-8 5 15 3-7h6"
              fill="none"
              stroke="currentColor"
              strokeWidth="2.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          <h1>PingBoard</h1>
        </div>
        <p className="tagline">Pantau uptime situs web secara real-time.</p>
      </header>

      {connectionError && (
        <div className="banner" role="alert">
          Tidak dapat terhubung ke server. Mencoba lagi otomatis…
        </div>
      )}

      <Summary {...stats} />

      <AddMonitorForm onCreate={handleCreate} />

      <main>
        {monitors === null && !connectionError && (
          <p className="empty">Memuat…</p>
        )}
        {monitors !== null && monitors.length === 0 && (
          <p className="empty">
            Belum ada monitor. Tambahkan URL pertama kamu di atas.
          </p>
        )}
        <ul className="grid">
          {(monitors ?? []).map((m) => (
            <li key={m.id}>
              <MonitorCard
                monitor={m}
                onDelete={handleDelete}
                onCheck={handleCheck}
              />
            </li>
          ))}
        </ul>
      </main>

      <footer className="footer">
        Dibangun dengan Go (goroutine, net/http) dan React. Data disimpan di
        memori dan hilang saat server restart.
      </footer>
    </div>
  )
}
