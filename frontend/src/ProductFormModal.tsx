import { useState, type FormEvent } from 'react'
import { api, errorLines } from './api'
import Modal from './Modal'
import type { Product } from './types'
import { parsePrice } from './validate'

interface Props{
  product:Product | null
  onClose:() => void
  onSaved:() => void
}

export default function ProductFormModal({ product, onClose, onSaved }: Props) {
  const [sku, setSku] = useState(product?.sku ?? '')
  const [name, setName] = useState(product?.name ?? '')
  const [price, setPrice] = useState(product ? String(product.price) : '')
  const [stock, setStock] = useState('0')
  const [active, setActive] = useState(product?.is_active ?? true)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<string[]>([])

  async function submit(e: FormEvent) {
    e.preventDefault()
    const p = parsePrice(price)
    const openingStock = parsePrice(stock)
    if (p === null) {
      setErrors(['Harga harus bilangan bulat >= 0'])
      return
    }
    if (openingStock === null) {
      setErrors(['Stok awal harus bilangan bulat >= 0'])
      return
    }
    setSaving(true)
    setErrors([])
    try {
      if (product) {
        await api.updateProduct(product.id, { sku, name, price: p, is_active: active })
      } else {
        await api.createProduct({ sku, name, price: p, is_active: active, stock: openingStock })
      }
      onSaved()
    } catch (err) {
      setErrors(errorLines(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal title={product ? `Edit ${product.sku}` : 'Produk baru'} onClose={onClose}>
      <form onSubmit={submit}>
        <label>SKU<input value={sku} onChange={(e) => setSku(e.target.value)} /></label>
        <label>Nama<input value={name} onChange={(e) => setName(e.target.value)} /></label>
        <label>Harga (Rp)<input value={price} onChange={(e) => setPrice(e.target.value)} inputMode="numeric" /></label>
        {!product && (
          <label>Stok awal<input value={stock} onChange={(e) => setStock(e.target.value)} inputMode="numeric" /></label>
        )}
        <label className="check">
          <input type="checkbox" checked={active} onChange={(e) => setActive(e.target.checked)} /> Aktif
        </label>
        {errors.map((m) => <p className="error" key={m}>{m}</p>)}
        <div className="row">
          <button type="button" onClick={onClose}>Batal</button>
          <button type="submit" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
        </div>
      </form>
    </Modal>
  )
}
