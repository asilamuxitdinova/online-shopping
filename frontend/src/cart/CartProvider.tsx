import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api } from '../api/api'
import type { Cart } from '../api/types'
import { useAuth } from '../auth/AuthProvider'

interface CartState {
  /** Null while logged out or before the cart has loaded. */
  cart: Cart | null
  count: number
  reload: () => Promise<void>
  add: (productId: number, quantity: number) => Promise<void>
  setQuantity: (productId: number, quantity: number) => Promise<void>
  remove: (productId: number) => Promise<void>
}

const CartContext = createContext<CartState | null>(null)

export function CartProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  const [cart, setCart] = useState<Cart | null>(null)

  const reload = useCallback(async () => {
    setCart(await api.cart())
  }, [])

  useEffect(() => {
    if (user) reload().catch(() => setCart(null))
    else setCart(null)
  }, [user, reload])

  const value = useMemo<CartState>(
    () => ({
      cart,
      count: cart?.count ?? 0,
      reload,
      add: async (productId, quantity) => setCart(await api.addToCart(productId, quantity)),
      setQuantity: async (productId, quantity) => setCart(await api.setCartQuantity(productId, quantity)),
      remove: async (productId) => setCart(await api.removeFromCart(productId)),
    }),
    [cart, reload],
  )

  return <CartContext.Provider value={value}>{children}</CartContext.Provider>
}

export function useCart(): CartState {
  const ctx = useContext(CartContext)
  if (!ctx) throw new Error('useCart must be used inside CartProvider')
  return ctx
}
