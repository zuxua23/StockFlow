const MAX_INT32 = 2147483647

export function parsePositiveInt(raw: string): number | null {
  const s = raw.trim()
  if (!/^\d+$/.test(s)) return null
  const n = Number(s)
  return n > 0 && n <= MAX_INT32 ? n : null
}

export function parsePrice(raw: string): number | null {
  const s = raw.trim()
  if (!/^\d+$/.test(s)) return null
  const n = Number(s)
  return n <= MAX_INT32 ? n : null
}

export interface OrderLine{
  sku:string
  qty:string
}

export interface OrderItemInput{
  sku:string
  qty:number
}

export function buildOrderItems(
  customer: string,
  lines: OrderLine[],
): { items: OrderItemInput[]; errors: string[] } {
  const errors: string[] = []
  if (customer.trim() === '') errors.push('Customer wajib diisi')

  const items: OrderItemInput[] = []
  const seen = new Set<string>()
  lines.forEach((l, i) => {
    const row = `Baris ${i + 1}`
    const sku = l.sku.trim()
    const qty = parsePositiveInt(l.qty)
    const duplicate = seen.has(sku)
    if (sku === '') errors.push(`${row}: SKU wajib diisi`)
    else if (duplicate) errors.push(`${row}: SKU ${sku} duplikat, gabungkan qty-nya`)
    seen.add(sku)
    if (qty === null) errors.push(`${row}: qty harus bilangan bulat > 0`)
    if (sku !== '' && !duplicate && qty !== null) items.push({ sku, qty })
  })
  return { items, errors }
}
