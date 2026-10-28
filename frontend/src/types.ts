export type StatusFilter = 'all' | 'active' | 'inactive' | 'low'
export type StockType = 'in' | 'out'

export interface Product{
    id:number
    sku:string
    name:string
    price:number
    stock:number
    is_active:boolean
    created_at:string
    last_movement_at:string | null
}

export interface ProductPage{
    data:Product[]
    page:number
    limit:number
    total:number
}

export interface ProductInput{
    sku:string
    name:string
    price:number
    stock?:number
    is_active?:boolean
}

export interface Summary{
    total_products:number
    active_products:number
    total_stock_value:number
    low_stock_count:number
}

export interface Movement{
    id:number
    product_id:number
    type:StockType
    qty:number
    note:string | null
    created_at:string
}

export interface AdjustResult{
    product:Product
    movement:Movement
}

export interface OrderItem{
    sku:string
    name:string
    qty:number
    unit_price:number
    subtotal:number
}

export interface Order{
    id:number
    customer:string
    status:'CREATED' | 'CANCELLED'
    total:number
    items:OrderItem[]
    created_at:string
    cancelled_at:string | null
}

export interface FieldError{
    field:string
    message:string
}
