export default function Summary({ total, up, down, avgUptime }) {
  return (
    <section className="summary" aria-label="Ringkasan">
      <Stat label="Total monitor" value={total} />
      <Stat label="Online" value={up} tone="up" />
      <Stat label="Offline" value={down} tone="down" />
      <Stat
        label="Rata-rata uptime"
        value={avgUptime === null ? '–' : `${avgUptime.toFixed(1)}%`}
      />
    </section>
  )
}

function Stat({ label, value, tone }) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <span className={`stat-value${tone ? ` tone-${tone}` : ''}`}>{value}</span>
    </div>
  )
}
