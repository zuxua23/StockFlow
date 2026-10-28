import { useEffect, useState } from 'react'
import { api, errorLines } from './api'
import { rupiah } from './format'
import type { Summary } from './types'

export default function SummaryCards({ version }: { version: number }) {
  const [s, setS] = useState<Summary | null>(null)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    api.productSummary()
      .then((d) => { if (!cancelled) { setS(d); setErr(null) } })
      .catch((e) => { if (!cancelled) setErr(errorLines(e)[0]) })
    return () => { cancelled = true }
  }, [version])

  if (err) return <p className="error">Ringkasan gagal dimuat: {err}</p>
  const cards: [string, string][] = [
    ['Total produk', s ? String(s.total_products) : '…'],
    ['Produk aktif', s ? String(s.active_products) : '…'],
    ['Nilai stok', s ? rupiah(s.total_stock_value) : '…'],
    ['Stok menipis (≤ 10)', s ? String(s.low_stock_count) : '…'],
  ]
  return (
    <div className="cards">
      {cards.map(([label, value]) => (
        <div className="card" key={label}>
          <div className="label">{label}</div>
          <div className="value">{value}</div>
        </div>
      ))}
    </div>
  )
}
