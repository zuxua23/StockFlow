import { useEffect, useState } from 'react'
import { ApiError, api, errorLines } from './api'
import AdjustStockModal from './AdjustStockModal'
import { rupiah, timeAgo } from './format'
import ProductFormModal from './ProductFormModal'
import SummaryCards from './SummaryCards'
import type { Product, ProductPage, StatusFilter } from './types'
import { useDebounce } from './useDebounce'

const LIMIT = 10
const FILTERS: [StatusFilter, string][] = [
  ['all', 'Semua'], ['active', 'Aktif'], ['inactive', 'Nonaktif'], ['low', 'Stok menipis'],
]

interface Props{
  version:number
  onChanged:() => void
}

export default function ProductsPage({ version, onChanged }: Props) {
  const [search, setSearch] = useState('')
  const debounced = useDebounce(search)
  const [status, setStatus] = useState<StatusFilter>('all')
  const [page, setPage] = useState(1)
  const [data, setData] = useState<ProductPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [tick, setTick] = useState(0)
  const [notice, setNotice] = useState<string | null>(null)
  const [adjusting, setAdjusting] = useState<Product | null>(null)
  const [editing, setEditing] = useState<Product | 'new' | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(null)
    api.listProducts({ search: debounced, status, page, limit: LIMIT })
      .then((d) => { if (!cancelled) setData(d) })
      .catch((e) => { if (!cancelled) setError(errorLines(e)[0]) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [debounced, status, page, tick, version])

  async function remove(p: Product) {
    if (!window.confirm(`Hapus ${p.sku}?`)) return
    try {
      await api.deleteProduct(p.id)
      setNotice(null)
      onChanged()
    } catch (e) {
      setNotice(
        e instanceof ApiError && e.code === 'PRODUCT_HAS_HISTORY'
          ? `${p.sku} punya riwayat stok dan tidak bisa dihapus. Nonaktifkan saja.`
          : errorLines(e)[0],
      )
    }
  }

  const totalPages = data ? Math.max(1, Math.ceil(data.total / LIMIT)) : 1

  return (
    <div>
      <SummaryCards version={version} />

      <div className="toolbar">
        <input
          placeholder="Cari SKU atau nama…"
          value={search}
          onChange={(e) => { setSearch(e.target.value); setPage(1) }}
        />
        <select value={status} onChange={(e) => { setStatus(e.target.value as StatusFilter); setPage(1) }}>
          {FILTERS.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
        </select>
        <button onClick={() => setEditing('new')}>+ Produk</button>
      </div>

      {notice && <p className="error" role="alert">{notice}</p>}

      {loading && <p>Memuat…</p>}
      {error && (
        <p className="error" role="alert">
          Gagal memuat produk: {error} <button onClick={() => setTick((t) => t + 1)}>Coba lagi</button>
        </p>
      )}
      {!loading && !error && data && data.data.length === 0 && <p>Tidak ada produk yang cocok.</p>}

      {!error && data && data.data.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>SKU</th><th>Nama</th><th className="num">Harga</th><th className="num">Stok</th>
              <th>Status</th><th>Gerakan terakhir</th><th>Aksi</th>
            </tr>
          </thead>
          <tbody>
            {data.data.map((p) => (
              <tr key={p.id}>
                <td>{p.sku}</td>
                <td>{p.name}</td>
                <td className="num">{rupiah(p.price)}</td>
                <td className={'num' + (p.stock <= 10 ? ' low' : '')}>{p.stock}</td>
                <td><span className={'badge ' + (p.is_active ? 'on' : 'off')}>{p.is_active ? 'Aktif' : 'Nonaktif'}</span></td>
                <td>{timeAgo(p.last_movement_at)}</td>
                <td className="actions">
                  <button onClick={() => setAdjusting(p)}>Stok</button>
                  <button onClick={() => setEditing(p)}>Edit</button>
                  <button onClick={() => remove(p)}>Hapus</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <div className="toolbar">
        <button disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>‹</button>
        <span>Hal {page} / {totalPages}</span>
        <button disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>›</button>
      </div>

      {adjusting && (
        <AdjustStockModal
          product={adjusting}
          onClose={() => setAdjusting(null)}
          onDone={() => { setAdjusting(null); onChanged() }}
        />
      )}
      {editing && (
        <ProductFormModal
          product={editing === 'new' ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); onChanged() }}
        />
      )}
    </div>
  )
}
