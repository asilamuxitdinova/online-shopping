import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router'
import { api } from '../api/api'
import { useCart } from '../cart/CartProvider'
import { Loading, Notice, ProductImage, QuantityStepper } from '../components/ui'
import { useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'

const MAX_PER_ORDER = 1000
const MAX_ADDRESS = 500

export function CartPage() {
  const { t, price, error: message } = useI18n()
  const { cart, reload, setQuantity, remove } = useCart()
  const navigate = useNavigate()
  const [busy, setBusy] = useState<number | 'checkout' | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [address, setAddress] = useState('')
  useTitle(t('cart.title'))

  // Stock may have changed since the cart was last loaded.
  useEffect(() => {
    reload().catch((err) => setError(message(err)))
    // Only on first render; reload and message change identity with the language.
  }, [])

  const run = async (key: number | 'checkout', action: () => Promise<void>) => {
    setBusy(key)
    setError(null)
    try {
      await action()
    } catch (err) {
      setError(message(err))
      reload().catch(() => {})
    } finally {
      setBusy(null)
    }
  }

  const checkout = (e: FormEvent) => {
    e.preventDefault()
    if (!address.trim()) {
      setError(t('cart.addressRequired'))
      return
    }
    run('checkout', async () => {
      const order = await api.checkout(address.trim())
      await reload()
      navigate(`/orders/${order.id}`, { state: { placed: true } })
    })
  }

  if (!cart) return error ? <Notice kind="error">{error}</Notice> : <Loading />

  if (cart.items.length === 0) {
    return (
      <div className="empty card">
        <h1>{t('cart.title')}</h1>
        <p className="muted">{t('cart.empty')}</p>
        <Link to="/" className="btn btn-primary">
          {t('cart.continue')}
        </Link>
      </div>
    )
  }

  return (
    <>
      <h1>{t('cart.title')}</h1>
      {error && <Notice kind="error">{error}</Notice>}
      <div className="cart-layout">
        <ul className="card cart-items">
          {cart.items.map((item) => (
            <li key={item.product_id} className="cart-item">
              <Link to={`/products/${item.product_id}`} className="cart-item-image" tabIndex={-1}>
                <ProductImage id={item.product_id} name={item.name} url={item.image_url} />
              </Link>
              <div className="cart-item-info">
                <Link to={`/products/${item.product_id}`} className="cart-item-name">
                  {item.name}
                </Link>
                <span className="muted">{price(item.price)}</span>
              </div>
              <QuantityStepper
                value={item.quantity}
                max={Math.min(item.stock, MAX_PER_ORDER)}
                disabled={busy !== null}
                onChange={(q) => run(item.product_id, () => setQuantity(item.product_id, q))}
              />
              <strong className="cart-item-subtotal">{price(item.subtotal)}</strong>
              <button
                type="button"
                className="link-button danger"
                disabled={busy !== null}
                onClick={() => run(item.product_id, () => remove(item.product_id))}
              >
                {t('cart.remove')}
              </button>
            </li>
          ))}
        </ul>

        <form className="card summary" onSubmit={checkout}>
          <h2>{t('cart.summary')}</h2>
          <dl className="summary-rows">
            <div>
              <dt>{t('cart.items')}</dt>
              <dd>{cart.count}</dd>
            </div>
            <div className="summary-total">
              <dt>{t('cart.total')}</dt>
              <dd>{price(cart.total)}</dd>
            </div>
          </dl>
          <label className="field">
            <span>{t('cart.address')}</span>
            <textarea
              rows={3}
              maxLength={MAX_ADDRESS}
              placeholder={t('cart.addressPlaceholder')}
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              autoComplete="street-address"
            />
          </label>
          <button type="submit" className="btn btn-primary btn-block" disabled={busy !== null}>
            {t('cart.placeOrder')}
          </button>
        </form>
      </div>
    </>
  )
}
