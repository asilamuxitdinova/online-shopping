import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { api } from '../api/api'
import { ApiError } from '../api/client'
import { AddToCartButton } from '../components/ProductCard'
import { LoadError, Loading, Notice, ProductImage, QuantityStepper, StockLabel } from '../components/ui'
import { useAsync, useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'

const MAX_PER_ORDER = 1000

export function ProductPage() {
  const { t, price, category } = useI18n()
  const id = Number(useParams().id)
  const product = useAsync(() => api.product(id), [id])
  const [quantity, setQuantity] = useState(1)
  const [added, setAdded] = useState(false)
  useTitle(product.data?.name)

  const back = (
    <Link to="/" className="back-link">
      ← {t('product.back')}
    </Link>
  )

  if (product.error) {
    const notFound = product.error instanceof ApiError && product.error.status === 404
    return (
      <>
        {back}
        {notFound ? <Notice>{t('product.notFound')}</Notice> : <LoadError error={product.error} onRetry={product.reload} />}
      </>
    )
  }
  const p = product.data
  if (!p) return <Loading />

  const max = Math.min(p.stock, MAX_PER_ORDER)

  return (
    <>
      {back}
      <article className="product-detail">
        <ProductImage id={p.id} name={p.name} url={p.image_url} className="product-detail-image" />
        <div className="product-detail-info">
          <span className="eyebrow">{category(p.category)}</span>
          <h1>{p.name}</h1>
          <p className="price price-lg">{price(p.price)}</p>
          <StockLabel stock={p.stock} />
          {p.description && <p className="description">{p.description}</p>}

          {p.stock > 0 && (
            <div className="buy-row">
              <QuantityStepper value={Math.min(quantity, max)} max={max} onChange={setQuantity} />
              <AddToCartButton product={p} quantity={Math.min(quantity, max)} onAdded={() => setAdded(true)} />
            </div>
          )}
          {added && (
            <Notice kind="success">
              {t('product.added')}. <Link to="/cart">{t('product.goToCart')} →</Link>
            </Notice>
          )}
        </div>
      </article>
    </>
  )
}
