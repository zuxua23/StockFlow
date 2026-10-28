import { useEffect, useState } from 'react'
import { api, errorLines } from './api'
import { rupiah } from './format'
import type { Order } from './types'

interface Props{
  id:number
  version:number
  onChanged:() => void
}

export default function OrderDetail({ id, version, onChanged }: Props) {
  const [order, setOrder] = useState<Order | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    api.getOrder(id)
      .then((o) => { if (!cancelled) { setOrder(o); setError(null) } })
      .catch((e) => { if (!cancelled) setError(errorLines(e)[0]) })
    return () => { cancelled = true }
  }, [id, version])

  async function cancel() {
    if (!window.confirm(`Batalkan order #${id}? Stok akan dikembalikan.`)) return
    setBusy(true)
    try {
      setOrder(await api.cancelOrder(id))
      setError(null)
      onChanged()
    } catch (e) {
      setError(errorLines(e)[0])
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="panel">
      <h3>Order #{id}</h3>
      {error && <p className="error" role="alert">{error}</p>}
      {order && (
        <>
          <p>{order.customer} · <span className={'badge ' + (order.status === 'CREATED' ? 'on' : 'off')}>{order.status}</span></p>
          <table>
            <thead><tr><th>SKU</th><th>Nama</th><th className="num">Qty</th><th className="num">Harga</th><th className="num">Subtotal</th></tr></thead>
            <tbody>
              {order.items.map((it) => (
                <tr key={it.sku}>
                  <td>{it.sku}</td><td>{it.name}</td>
                  <td className="num">{it.qty}</td>
                  <td className="num">{rupiah(it.unit_price)}</td>
                  <td className="num">{rupiah(it.subtotal)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <p>Total: <b>{rupiah(order.total)}</b></p>
          {order.status === 'CREATED' && (
            <button disabled={busy} onClick={cancel}>{busy ? 'Membatalkan…' : 'Batalkan order'}</button>
          )}
        </>
      )}
    </div>
  )
}
