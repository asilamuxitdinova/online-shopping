import { request, tokens } from './client'
import type {
  AuthResponse,
  Cart,
  CategoryCount,
  Order,
  OrderStatus,
  Product,
  ProductInput,
  ProductPage,
  User,
} from './types'

export interface ProductQuery {
  search?: string
  category?: string
  minPrice?: string
  maxPrice?: string
  sort?: string
  page?: number
  limit?: number
}

function queryString(params: Record<string, string | number | undefined>): string {
  const q = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') q.set(key, String(value))
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}

export const api = {
  register: (name: string, email: string, password: string) =>
    request<AuthResponse>('/api/auth/register', { method: 'POST', body: { name, email, password }, auth: false }),
  login: (email: string, password: string) =>
    request<AuthResponse>('/api/auth/login', { method: 'POST', body: { email, password }, auth: false }),
  logout: () =>
    request<void>('/api/auth/logout', { method: 'POST', body: { refreshToken: tokens.refresh }, auth: false }),
  me: () => request<{ user: User }>('/api/auth/me').then((r) => r.user),

  products: (query: ProductQuery) =>
    request<ProductPage>('/api/products' + queryString({ ...query })),
  categories: () =>
    request<{ categories: CategoryCount[] }>('/api/products/categories').then((r) => r.categories),
  product: (id: number) => request<{ product: Product }>(`/api/products/${id}`).then((r) => r.product),
  createProduct: (input: ProductInput) =>
    request<{ product: Product }>('/api/products', { method: 'POST', body: input }).then((r) => r.product),
  updateProduct: (id: number, input: ProductInput) =>
    request<{ product: Product }>(`/api/products/${id}`, { method: 'PATCH', body: input }).then((r) => r.product),
  deleteProduct: (id: number) => request<void>(`/api/products/${id}`, { method: 'DELETE' }),

  cart: () => request<Cart>('/api/cart'),
  addToCart: (productId: number, quantity: number) =>
    request<Cart>('/api/cart/items', { method: 'POST', body: { productId, quantity } }),
  setCartQuantity: (productId: number, quantity: number) =>
    request<Cart>(`/api/cart/items/${productId}`, { method: 'PATCH', body: { quantity } }),
  removeFromCart: (productId: number) => request<Cart>(`/api/cart/items/${productId}`, { method: 'DELETE' }),

  checkout: (shippingAddress: string) =>
    request<{ order: Order }>('/api/orders', { method: 'POST', body: { shippingAddress } }).then((r) => r.order),
  orders: (all = false) =>
    request<{ orders: Order[] }>('/api/orders' + (all ? '?all=true' : '')).then((r) => r.orders),
  order: (id: number) => request<{ order: Order }>(`/api/orders/${id}`).then((r) => r.order),
  cancelOrder: (id: number) =>
    request<{ order: Order }>(`/api/orders/${id}/cancel`, { method: 'POST' }).then((r) => r.order),
  setOrderStatus: (id: number, status: OrderStatus) =>
    request<{ order: Order }>(`/api/orders/${id}/status`, { method: 'PATCH', body: { status } }).then((r) => r.order),
}
