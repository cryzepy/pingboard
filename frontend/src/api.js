// Jika frontend di-host terpisah dari backend, set VITE_API_URL saat build,
// misalnya: VITE_API_URL=https://api.contoh.com npm run build
const BASE = (import.meta.env.VITE_API_URL ?? '') + '/api'

async function request(path, options = {}) {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (res.status === 204) return null

  let data = null
  try {
    data = await res.json()
  } catch {
    // respons bukan JSON
  }
  if (!res.ok) {
    throw new Error(data?.error ?? `Permintaan gagal (${res.status})`)
  }
  return data
}

export const listMonitors = (signal) => request('/monitors', { signal })

export const createMonitor = (body) =>
  request('/monitors', { method: 'POST', body: JSON.stringify(body) })

export const deleteMonitor = (id) =>
  request(`/monitors/${id}`, { method: 'DELETE' })

export const checkNow = (id) =>
  request(`/monitors/${id}/check`, { method: 'POST' })
