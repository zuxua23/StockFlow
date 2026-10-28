import { useEffect, useState } from 'react'
import { api, errorLines } from './api'
import CreateOrderForm from './CreateOrderForm'
import { rupiah } from './format'
import OrderDetail from './OrderDetail'
import type { Order } from './types'

export default function OrdersPage({ onChanged }: { onChanged: () => void }) {
  const [orders, setOrders] = useState<Order[]>([])
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<number | null>(null)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    api.listOrders()
      .then((r) => { if (!cancelled) { setOrders(r.data); setError(null) } })
      .catch((e) => { if (!cancelled) setError(errorLines(e)[0]) })
    return () => { cancelled = true }
  }, [tick])

  function changed() {
    setTick((t) => t + 1)
    onChanged()
  }

  return (
    <div className="two-col">
      <div>
        <CreateOrderForm onCreated={changed} />
        <div className="panel">
          <h3>Order terbaru</h3>
          {error && <p className="error">{error} <button onClick={() => setTick((t) => t + 1)}>Coba lagi</button></p>}
          {orders.length === 0 && !error && <p>Belum ada order.</p>}
          <ul className="orders">
            {orders.map((o) => (
              <li key={o.id}>
                <button onClick={() => setSelected(o.id)}>
                  #{o.id} · {o.customer} · {rupiah(o.total)} · {o.status}
                </button>
              </li>
            ))}
          </ul>
        </div>
      </div>
      <div>
        {selected === null
          ? <p>Pilih order untuk melihat detail.</p>
          : <OrderDetail id={selected} version={tick} onChanged={changed} />}
      </div>
    </div>
  )
}
