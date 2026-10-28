import { useState, type FormEvent } from 'react'
import { ApiError, api, errorLines } from './api'
import Modal from './Modal'
import type { Product, StockType } from './types'
import { parsePositiveInt } from './validate'

interface Props{
  product:Product
  onClose:() => void
  onDone:() => void
}

export default function AdjustStockModal({ product, onClose, onDone }: Props) {
  const [type, setType] = useState<StockType>('in')
  const [qty, setQty] = useState('')
  const [note, setNote] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const n = parsePositiveInt(qty)
  const valid = n !== null

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (n === null) return
    setSaving(true)
    setError(null)
    try {
      await api.adjustStock(product.id, { type, qty: n, note })
      onDone()
    } catch (err) {
      if (err instanceof ApiError && err.code === 'INSUFFICIENT_STOCK') {
        setError(`Stok tidak cukup. Tersedia: ${err.available ?? 0}`)
      } else {
        setError(errorLines(err).join('. '))
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={`Atur stok: ${product.sku}`} onClose={onClose}>
      <p>Stok saat ini: <b>{product.stock}</b></p>
      <form onSubmit={submit}>
        <label>
          Arah
          <select value={type} onChange={(e) => setType(e.target.value as StockType)}>
            <option value="in">Masuk (in)</option>
            <option value="out">Keluar (out)</option>
          </select>
        </label>
        <label>
          Qty
          <input value={qty} onChange={(e) => setQty(e.target.value)} inputMode="numeric" />
        </label>
        {qty !== '' && !valid && <p className="error">Qty harus bilangan bulat &gt; 0</p>}
        <label>
          Catatan
          <input value={note} onChange={(e) => setNote(e.target.value)} />
        </label>
        {error && <p className="error" role="alert">{error}</p>}
        <div className="row">
          <button type="button" onClick={onClose}>Batal</button>
          <button type="submit" disabled={!valid || saving}>
            {saving ? 'Menyimpan…' : 'Simpan'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
