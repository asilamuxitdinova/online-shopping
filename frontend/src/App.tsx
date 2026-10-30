import { BrowserRouter, Route, Routes } from 'react-router'
import { AuthProvider } from './auth/AuthProvider'
import { CartProvider } from './cart/CartProvider'
import { Layout } from './components/Layout'
import { RequireAuth } from './components/RequireAuth'
import { I18nProvider } from './i18n/I18nProvider'
import { AdminPage } from './pages/AdminPage'
import { LoginPage, RegisterPage } from './pages/AuthPages'
import { CartPage } from './pages/CartPage'
import { CatalogPage } from './pages/CatalogPage'
import { NotFoundPage } from './pages/NotFoundPage'
import { OrderPage, OrdersPage } from './pages/OrdersPage'
import { ProductPage } from './pages/ProductPage'

export function App() {
  return (
    <BrowserRouter>
      <I18nProvider>
        <AuthProvider>
          <CartProvider>
            <Layout>
              <Routes>
                <Route path="/" element={<CatalogPage />} />
                <Route path="/products/:id" element={<ProductPage />} />
                <Route path="/login" element={<LoginPage />} />
                <Route path="/register" element={<RegisterPage />} />
                <Route element={<RequireAuth />}>
                  <Route path="/cart" element={<CartPage />} />
                  <Route path="/orders" element={<OrdersPage />} />
                  <Route path="/orders/:id" element={<OrderPage />} />
                </Route>
                <Route element={<RequireAuth admin />}>
                  <Route path="/admin" element={<AdminPage />} />
                </Route>
                <Route path="*" element={<NotFoundPage />} />
              </Routes>
            </Layout>
          </CartProvider>
        </AuthProvider>
      </I18nProvider>
    </BrowserRouter>
  )
}
