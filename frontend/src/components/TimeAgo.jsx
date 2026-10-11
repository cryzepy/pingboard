import { useEffect, useState } from 'react'

function format(iso, now) {
  const seconds = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000))
  if (seconds < 5) return 'baru saja'
  if (seconds < 60) return `${seconds} dtk lalu`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} mnt lalu`
  return `${Math.floor(minutes / 60)} jam lalu`
}

export default function TimeAgo({ iso }) {
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [])

  return <time dateTime={iso}>{format(iso, now)}</time>
}
