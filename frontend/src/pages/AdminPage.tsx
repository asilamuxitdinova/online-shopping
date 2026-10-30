import { useState, type FormEvent } from 'react'
import { Link, useSearchParams } from 'react-router'
import { api } from '../api/api'
import { orderStatuses, type Order, type OrderStatus, type Product, type ProductInput } from '../api/types'
import { LoadError, Loading, Notice, Pagination, ProductImage, StatusBadge } from '../components/ui'
import { useAsync, useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'

export function AdminPage() {
  const { t } = useI18n()
  const [params, setParams] = useSearchParams()
  const tab = params.get('tab') === 'orders' ? 'orders' : 'products'
  useTitle(t('admin.title'))

  return (
    <>
      <h1>{t('admin.title')}</h1>
      <div className="tabs" role="tablist">
        <button type="button" role="tab" aria-selected={tab === 'products'} onClick={() => setParams({})}>
          {t('admin.products')}
        </button>
        <button type="button" role="tab" aria-selected={tab === 'orders'} onClick={() => setParams({ tab: 'orders' })}>
          {t('admin.orders')}
        </button>
      </div>
      {tab === 'products' ? <ProductsTab /> : <OrdersTab />}
    </>
  )
}

const PAGE_SIZE = 20

function ProductsTab() {
  const { t, price, category, error: message } = useI18n()
  const [page, setPage] = useState(1)
  const products = useAsync(() => api.products({ page, limit: PAGE_SIZE }), [page])
  // null: form closed, 'new': creating, a product: editing it.
  const [editing, setEditing] = useState<Product | 'new' | null>(null)
  const [notice, setNotice] = useState<{ kind: 'success' | 'error'; text: string } | null>(null)

  const remove = async (p: Product) => {
    if (!window.confirm(t('admin.deleteConfirm', { name: p.name }))) return
    try {
      await api.deleteProduct(p.id)
      setNotice({ kind: 'success', text: t('admin.saved') })
      products.reload()
    } catch (err) {
      setNotice({ kind: 'error', text: message(err) })
    }
  }

  return (
    <section>
      <div className="toolbar">
        <span />
        <button type="button" className="btn btn-primary" onClick={() => setEditing('new')}>
          + {t('admin.newProduct')}
        </button>
      </div>
      {notice && <Notice kind={notice.kind}>{notice.text}</Notice>}
      {editing && (
        <ProductForm
          product={editing === 'new' ? null : editing}
          onCancel={() => setEditing(null)}
          onSaved={() => {
            setEditing(null)
            setNotice({ kind: 'success', text: t('admin.saved') })
            products.reload()
          }}
        />
      )}

      {products.error ? (
        <LoadError error={products.error} onRetry={products.reload} />
      ) : !products.data ? (
        <Loading />
      ) : products.data.items.length === 0 ? (
        <p className="muted">{t('admin.noProducts')}</p>
      ) : (
        <div className="card table-wrap">
          <table className="table">
            <thead>
              <tr>
                <th>#</th>
                <th>{t('admin.name')}</th>
                <th>{t('admin.category')}</th>
                <th className="num">{t('admin.price')}</th>
                <th className="num">{t('admin.stock')}</th>
                <th>{t('admin.actions')}</th>
              </tr>
            </thead>
            <tbody>
              {products.data.items.map((p) => (
                <tr key={p.id}>
                  <td className="muted">{p.id}</td>
                  <td>
                    <div className="admin-product">
                      <ProductImage id={p.id} name={p.name} url={p.image_url} className="thumb" />
                      <Link to={`/products/${p.id}`}>{p.name}</Link>
                    </div>
                  </td>
                  <td>{category(p.category)}</td>
                  <td className="num">{price(p.price)}</td>
                  <td className={`num ${p.stock === 0 ? 'danger' : ''}`}>{p.stock}</td>
                  <td>
                    <div className="actions">
                      <button type="button" className="btn btn-sm" onClick={() => setEditing(p)}>
                        {t('admin.edit')}
                      </button>
                      <button type="button" className="btn btn-sm btn-danger" onClick={() => remove(p)}>
                        {t('admin.delete')}
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {products.data && <Pagination page={products.data.page} pages={products.data.pages} onChange={setPage} />}
    </section>
  )
}

function ProductForm({ product, onCancel, onSaved }: { product: Product | null; onCancel: () => void; onSaved: () => void }) {
  const { t, error: message } = useI18n()
  const [fields, setFields] = useState({
    name: product?.name ?? '',
    description: product?.description ?? '',
    price: product ? String(product.price) : '',
    stock: product ? String(product.stock) : '0',
    category: product?.category ?? '',
    image_url: product?.image_url ?? '',
  })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const set = (key: keyof typeof fields) => (e: { target: { value: string } }) =>
    setFields((f) => ({ ...f, [key]: e.target.value }))

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    const input: ProductInput = {
      name: fields.name.trim(),
      description: fields.description,
      price: Number(fields.price),
      stock: Number(fields.stock),
      category: fields.category.trim(),
      image_url: fields.image_url.trim(),
    }
    setBusy(true)
    setError(null)
    try {
      if (product) await api.updateProduct(product.id, input)
      else await api.createProduct(input)
      onSaved()
    } catch (err) {
      setError(message(err))
      setBusy(false)
    }
  }

  return (
    <form className="card product-form" onSubmit={submit}>
      <h2>{product ? t('admin.editProduct') : t('admin.newProduct')}</h2>
      {error && <Notice kind="error">{error}</Notice>}
      <div className="form-grid">
        <label className="field span-2">
          <span>{t('admin.name')}</span>
          <input required maxLength={200} value={fields.name} onChange={set('name')} />
        </label>
        <label className="field">
          <span>{t('admin.price')}</span>
          <input type="number" required min={0} max={1_000_000_000} step="any" value={fields.price} onChange={set('price')} />
        </label>
        <label className="field">
          <span>{t('admin.stock')}</span>
          <input type="number" required min={0} max={1_000_000_000} step={1} value={fields.stock} onChange={set('stock')} />
        </label>
        <label className="field">
          <span>{t('admin.category')}</span>
          <input maxLength={200} value={fields.category} onChange={set('category')} placeholder="electronics" />
        </label>
        <label className="field">
          <span>{t('admin.imageUrl')}</span>
          <input type="url" maxLength={2048} value={fields.image_url} onChange={set('image_url')} placeholder="https://" />
        </label>
        <label className="field span-2">
          <span>{t('admin.description')}</span>
          <textarea rows={3} maxLength={5000} value={fields.description} onChange={set('description')} />
        </label>
      </div>
      <div className="form-actions">
        <button type="submit" className="btn btn-primary" disabled={busy}>
          {t('admin.save')}
        </button>
        <button type="button" className="btn" onClick={onCancel}>
          {t('admin.cancel')}
        </button>
      </div>
    </form>
  )
}

function OrdersTab() {
  const { t, price, date, error: message } = useI18n()
  const orders = useAsync(() => api.orders(true), [])
  const [error, setError] = useState<string | null>(null)

  const changeStatus = async (order: Order, status: OrderStatus) => {
    setError(null)
    try {
      const updated = await api.setOrderStatus(order.id, status)
      orders.setData((orders.data ?? []).map((o) => (o.id === updated.id ? updated : o)))
    } catch (err) {
      setError(message(err))
    }
  }

  if (orders.error) return <LoadError error={orders.error} onRetry={orders.reload} />
  if (!orders.data) return <Loading />
  if (orders.data.length === 0) return <p className="muted">{t('admin.noOrders')}</p>

  return (
    <section>
      {error && <Notice kind="error">{error}</Notice>}
      <div className="card table-wrap">
        <table className="table">
          <thead>
            <tr>
              <th>#</th>
              <th>{t('order.date')}</th>
              <th>{t('admin.customer')}</th>
              <th className="num">{t('order.total')}</th>
              <th>{t('order.status')}</th>
            </tr>
          </thead>
          <tbody>
            {orders.data.map((o) => (
              <tr key={o.id}>
                <td>
                  <Link to={`/orders/${o.id}`}>{o.id}</Link>
                </td>
                <td>{date(o.created_at)}</td>
                <td className="muted">{o.user_id}</td>
                <td className="num">{price(o.total)}</td>
                <td>
                  {o.status === 'cancelled' ? (
                    <StatusBadge status={o.status} />
                  ) : (
                    <select
                      aria-label={t('order.status')}
                      value={o.status}
                      onChange={(e) => changeStatus(o, e.target.value as OrderStatus)}
                    >
                      {orderStatuses.map((s) => (
                        <option key={s} value={s}>
                          {t(`status.${s}`)}
                        </option>
                      ))}
                    </select>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
