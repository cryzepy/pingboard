const WIDTH = 260
const HEIGHT = 48
const PAD = 4

// Grafik kecil latensi untuk riwayat pengecekan. Titik yang gagal (down)
// ditandai dengan kolom merah, jadi tidak hanya bergantung pada warna garis.
export default function Sparkline({ history }) {
  if (history.length === 0) {
    return <div className="spark spark--empty">Menunggu pengecekan pertama…</div>
  }

  const n = history.length
  const stepX = n > 1 ? (WIDTH - PAD * 2) / (n - 1) : 0
  const x = (i) => (n > 1 ? PAD + i * stepX : WIDTH / 2)

  const ups = history.map((c, i) => ({ ...c, i })).filter((c) => c.up)
  const maxLatency = Math.max(50, ...ups.map((c) => c.latencyMs))
  const y = (ms) =>
    HEIGHT - PAD - (ms / maxLatency) * (HEIGHT - PAD * 2)

  const points = ups.map((c) => `${x(c.i).toFixed(1)},${y(c.latencyMs).toFixed(1)}`)
  const last = ups.length > 0 && ups[ups.length - 1].i === n - 1 ? ups[ups.length - 1] : null
  const colW = Math.max(3, Math.min(8, stepX * 0.7 || 6))

  return (
    <svg
      className="spark"
      viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
      preserveAspectRatio="none"
      role="img"
      aria-label={`Grafik latensi ${n} pengecekan terakhir`}
    >
      {history.map(
        (c, i) =>
          !c.up && (
            <rect
              key={i}
              className="spark-down"
              x={x(i) - colW / 2}
              y={0}
              width={colW}
              height={HEIGHT}
              rx="1.5"
            />
          ),
      )}
      {points.length > 1 && (
        <polyline className="spark-line" points={points.join(' ')} />
      )}
      {last && <circle className="spark-dot" cx={x(last.i)} cy={y(last.latencyMs)} r="3" />}
    </svg>
  )
}
