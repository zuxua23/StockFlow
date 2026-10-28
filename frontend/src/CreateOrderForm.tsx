import { useEffect, useState, type FormEvent } from 'react'
import { ApiError, api, errorLines } from './api'
import { rupiah } from './format'
import type { Order, Product } from './types'
import { buildOrderItems } from './validate'

interface Line{
  sku:string
  qty:string
}

export default function CreateOrderForm({ onCreated }: { onCreated: () => void }) {
  const [customer, setCustomer] = useState('')
  const [lines, setLines] = useState<Line[]>([{ sku: '', qty: '1' }])
  const [products, setProducts] = useState<Product[]>([])
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<string[]>([])
  const [failed, setFailed] = useState<{ sku: string; available: number } | null>(null)
  const [created, setCreated] = useState<Order | null>(null)

  useEffect(() => {
    api.listProducts({ search: '', status: 'active', page: 1, limit: 100 })
      .then((r) => setProducts(r.data))
      .catch(() => setProducts([]))
  }, [])

  function setLine(i: number, patch: Partial<Line>) {
    setLines((ls) => ls.map((l, j) => (j === i ? { ...l, ...patch } : l)))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setErrors([])
    setFailed(null)
    setCreated(null)
    const { items, errors: problems } = buildOrderItems(customer, lines)
    if (problems.length > 0) {
      setErrors(problems)
      return
    }
    setSaving(true)
    try {
      const order = await api.createOrder({ customer: customer.trim(), items })
      setCreated(order)
      setCustomer('')
      setLines([{ sku: '', qty: '1' }])
      onCreated()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'INSUFFICIENT_STOCK' && err.sku !== undefined) {
        setFailed({ sku: err.sku, available: err.available ?? 0 })
      } else {
        setErrors(errorLines(err))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} className="panel">
      <h3>Order baru</h3>
      <label>Customer<input value={customer} onChange={(e) => setCustomer(e.target.value)} /></label>

      <datalist id="skus">
        {products.map((p) => <option key={p.sku} value={p.sku}>{p.name} (stok {p.stock})</option>)}
      </datalist>

      {lines.map((l, i) => {
        const bad = failed !== null && failed.sku === l.sku.trim()
        return (
          <div key={i}>
            <div className={'row' + (bad ? ' bad' : '')}>
              <input list="skus" placeholder="SKU" value={l.sku} onChange={(e) => setLine(i, { sku: e.target.value })} />
              <input className="qty" value={l.qty} onChange={(e) => setLine(i, { qty: e.target.value })} inputMode="numeric" />
              <button type="button" disabled={lines.length === 1} onClick={() => setLines((ls) => ls.filter((_, j) => j !== i))}>✕</button>
            </div>
            {bad && <p className="error">Stok {failed.sku} tidak cukup. Tersedia: {failed.available}</p>}
          </div>
        )
      })}

      <button type="button" onClick={() => setLines((ls) => [...ls, { sku: '', qty: '1' }])}>+ Item</button>
      {errors.map((m) => <p className="error" key={m}>{m}</p>)}
      <button type="submit" disabled={saving}>{saving ? 'Menyimpan…' : 'Buat order'}</button>

      {created && (
        <p className="ok">Order #{created.id} dibuat. Total (hitungan server): <b>{rupiah(created.total)}</b></p>
      )}
    </form>
  )
}
