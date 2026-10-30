import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import type { Product } from '../api/types'
import { useAuth } from '../auth/AuthProvider'
import { useCart } from '../cart/CartProvider'
import { useI18n } from '../i18n/I18nProvider'
import { CheckIcon } from './icons'
import { ProductImage, StockLabel } from './ui'

/**
 * Adds a product to the cart, sending logged-out visitors to the login page
 * first. Reports the outcome next to the button.
 */
export function AddToCartButton({
  product,
  quantity = 1,
  className = '',
  onAdded,
}: {
  product: Product
  quantity?: number
  className?: string
  onAdded?: () => void
}) {
  const { t, error: message } = useI18n()
  const { user } = useAuth()
  const { add } = useCart()
  const navigate = useNavigate()
  const location = useLocation()
  const [state, setState] = useState<'idle' | 'busy' | 'added'>('idle')
  const [error, setError] = useState<string | null>(null)

  const onClick = async () => {
    if (!user) {
      navigate('/login', { state: { from: location } })
      return
    }
    setState('busy')
    setError(null)
    try {
      await add(product.id, quantity)
      setState('added')
      onAdded?.()
      setTimeout(() => setState('idle'), 2000)
    } catch (err) {
      setState('idle')
      setError(message(err))
    }
  }

  return (
    <div className="add-to-cart">
      <button
        type="button"
        className={`btn btn-primary ${className}`}
        disabled={product.stock <= 0 || state === 'busy'}
        onClick={onClick}
      >
        {state === 'added' ? (
          <>
            <CheckIcon width={16} height={16} /> {t('product.added')}
          </>
        ) : (
          t('product.addToCart')
        )}
      </button>
      {error && (
        <p className="field-error" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}

export function ProductCard({ product }: { product: Product }) {
  const { price, category } = useI18n()
  const link = `/products/${product.id}`
  return (
    <article className="card product-card">
      <Link to={link} className="product-card-media" tabIndex={-1}>
        <ProductImage id={product.id} name={product.name} url={product.image_url} />
      </Link>
      <div className="product-card-body">
        <span className="eyebrow">{category(product.category)}</span>
        <h3 className="product-card-title">
          <Link to={link}>{product.name}</Link>
        </h3>
        <StockLabel stock={product.stock} />
        <div className="product-card-footer">
          <strong className="price">{price(product.price)}</strong>
          <AddToCartButton product={product} className="btn-sm" />
        </div>
      </div>
    </article>
  )
}
