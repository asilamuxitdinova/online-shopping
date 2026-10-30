// Shapes of the backend's JSON responses (see backend/README.md).

export type Role = 'customer' | 'admin'

export interface User {
  id: number
  name: string
  email: string
  role: Role
  created_at: string
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  token_type: 'Bearer'
  expires_in: number
  user: User
}

export interface Product {
  id: number
  name: string
  description: string
  price: number
  stock: number
  category: string
  image_url: string | null
  created_at: string
  updated_at: string
}

export interface ProductPage {
  items: Product[]
  page: number
  limit: number
  total: number
  pages: number
}

export interface CategoryCount {
  category: string
  count: number
}

export interface CartItem {
  product_id: number
  name: string
  price: number
  image_url: string | null
  stock: number
  quantity: number
  subtotal: number
}

export interface Cart {
  items: CartItem[]
  count: number
  total: number
}

export const orderStatuses = ['pending', 'paid', 'shipped', 'delivered', 'cancelled'] as const
export type OrderStatus = (typeof orderStatuses)[number]

export interface OrderItem {
  product_id: number | null
  name: string
  price: number
  quantity: number
  subtotal: number
}

export interface Order {
  id: number
  user_id: number
  status: OrderStatus
  total: number
  shipping_address: string
  created_at: string
  items: OrderItem[]
}

export interface ProductInput {
  name?: string
  description?: string
  price?: number
  stock?: number
  category?: string
  image_url?: string
}
