import { useCallback, useState } from 'react'
import OrdersPage from './OrdersPage'
import ProductsPage from './ProductsPage'

export default function App() {
  const [tab, setTab] = useState<'products' | 'orders'>('products')
  const [version, setVersion] = useState(0)
  const refresh = useCallback(() => setVersion((v) => v + 1), [])

  return (
    <div className="app">
      <header>
        <h1>StockFlow</h1>
        <nav>
          <button className={tab === 'products' ? 'active' : ''} onClick={() => setTab('products')}>Produk</button>
          <button className={tab === 'orders' ? 'active' : ''} onClick={() => setTab('orders')}>Order</button>
        </nav>
      </header>
      {tab === 'products'
        ? <ProductsPage version={version} onChanged={refresh} />
        : <OrdersPage onChanged={refresh} />}
    </div>
  )
}
