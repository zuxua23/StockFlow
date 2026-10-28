import type {
    AdjustResult, FieldError, Movement, Order, Product, ProductInput,
    ProductPage, StatusFilter, StockType, Summary,
  } from './types'

  export class ApiError extends Error {
    status: number
    code: string
    details: FieldError[]
    available?: number
    sku?: string

    constructor(
      status: number,
      code: string,
      message: string,
      extra: { details?: FieldError[]; available?: number; sku?: string } = {},
    ) {
      super(message)
      this.status = status
      this.code = code
      this.details = extra.details ?? []
      this.available = extra.available
      this.sku = extra.sku
    }
  }

  interface Envelope{
    error?:{
      code?:string
      message?:string
      details?:FieldError[]
      available?:number
      sku?:string
    }
  }

  async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    let res: Response
    try {
      res = await fetch('/api' + path, {
        ...init,
        headers: { 'Content-Type': 'application/json' },
      })
    } catch {
      throw new ApiError(0, 'NETWORK', 'Tidak bisa terhubung ke server')
    }
    if (res.status === 204) return undefined as T

    const text = await res.text()
    let body: unknown = null
    try {
      body = text ? JSON.parse(text) : null
    } catch {
      body = null
    }

    if (!res.ok) {
      const e = (body as Envelope | null)?.error
      throw new ApiError(res.status, e?.code ?? 'UNKNOWN', e?.message ?? `HTTP ${res.status}`, {
        details: e?.details,
        available: e?.available,
        sku: e?.sku,
      })
    }
    return body as T
  }

  const send = (method: string, body?: unknown): RequestInit => ({
    method,
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  export function errorLines(e: unknown): string[] {
    if (e instanceof ApiError) {
      return e.details.length ? e.details.map((d) => d.message) : [e.message]
    }
    return [e instanceof Error ? e.message : 'Terjadi kesalahan']
  }

  export const api = {
    listProducts(p: { search: string; status: StatusFilter; page: number; limit: number }) {
      const q = new URLSearchParams({
        page: String(p.page),
        limit: String(p.limit),
        status: p.status,
      })
      if (p.search) q.set('search', p.search)
      return request<ProductPage>(`/products?${q}`)
    },
    productSummary: () => request<Summary>('/products/summary'),
    createProduct: (b: ProductInput) => request<Product>('/products', send('POST', b)),
    updateProduct: (id: number, b: ProductInput) =>
      request<Product>(`/products/${id}`, send('PUT', b)),
    deleteProduct: (id: number) => request<void>(`/products/${id}`, send('DELETE')),
    adjustStock: (id: number, b: { type: StockType; qty: number; note: string }) =>
      request<AdjustResult>(`/products/${id}/stock`, send('POST', b)),
    listMovements: (id: number) =>
      request<{ data: Movement[] }>(`/products/${id}/movements`),

    listOrders: () => request<{ data: Order[] }>('/orders'),
    getOrder: (id: number) => request<Order>(`/orders/${id}`),
    createOrder: (b: { customer: string; items: { sku: string; qty: number }[] }) =>
      request<Order>('/orders', send('POST', b)),
    cancelOrder: (id: number) => request<Order>(`/orders/${id}/cancel`, send('POST')),
  }