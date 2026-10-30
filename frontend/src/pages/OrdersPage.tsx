import { useState } from 'react'
import { Link, useLocation, useParams } from 'react-router'
import { api } from '../api/api'
import { ApiError } from '../api/client'
import { useAuth } from '../auth/AuthProvider'
import { LoadError, Loading, Notice, StatusBadge } from '../components/ui'
import { useAsync, useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'

export function OrdersPage() {
  const { t, price, date } = useI18n()
  const orders = useAsync(() => api.orders(), [])
  useTitle(t('orders.title'))

  return (
    <>
      <h1>{t('orders.title')}</h1>
      {orders.error ? (
        <LoadError error={orders.error} onRetry={orders.reload} />
      ) : !orders.data ? (
        <Loading />
      ) : orders.data.length === 0 ? (
        <div className="empty card">
          <p className="muted">{t('orders.empty')}</p>
          <Link to="/" className="btn btn-primary">
            {t('cart.continue')}
          </Link>
        </div>
      ) : (
        <ul className="order-list">
          {orders.data.map((o) => (
            <li key={o.id}>
              <Link to={`/orders/${o.id}`} className="card order-row">
                <strong>{t('order.title', { id: o.id })}</strong>
                <span className="muted">{date(o.created_at)}</span>
                <span className="muted">{t('order.itemCount', { count: o.items.reduce((n, i) => n + i.quantity, 0) })}</span>
                <strong>{price(o.total)}</strong>
                <StatusBadge status={o.status} />
              </Link>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}

export function OrderPage() {
  const { t, price, date, error: message } = useI18n()
  const { user } = useAuth()
  const id = Number(useParams().id)
  const location = useLocation()
  const order = useAsync(() => api.order(id), [id])
  const [cancelError, setCancelError] = useState<string | null>(null)
  const [cancelling, setCancelling] = useState(false)
  useTitle(t('order.title', { id }))

  const back = (
    <Link to="/orders" className="back-link">
      ← {t('orders.back')}
    </Link>
  )

  if (order.error) {
    const notFound = order.error instanceof ApiError && order.error.status === 404
    return (
      <>
        {back}
        {notFound ? <Notice>{t('error.notFound')}</Notice> : <LoadError error={order.error} onRetry={order.reload} />}
      </>
    )
  }
  const o = order.data
  if (!o) return <Loading />

  const cancel = async () => {
    if (!window.confirm(t('order.cancelConfirm'))) return
    setCancelling(true)
    setCancelError(null)
    try {
      order.setData(await api.cancelOrder(o.id))
    } catch (err) {
      setCancelError(message(err))
    } finally {
      setCancelling(false)
    }
  }

  return (
    <>
      {back}
      {(location.state as { placed?: boolean } | null)?.placed && o.status === 'pending' && (
        <Notice kind="success">{t('order.placed')}</Notice>
      )}
      <div className="order-header">
        <h1>{t('order.title', { id: o.id })}</h1>
        <StatusBadge status={o.status} />
      </div>
      <p className="muted">
        {date(o.created_at)}
        {user?.role === 'admin' && user.id !== o.user_id && <> · {t('order.customer', { id: o.user_id })}</>}
      </p>

      <div className="card table-wrap">
        <table className="table">
          <thead>
            <tr>
              <th>{t('order.product')}</th>
              <th className="num">{t('order.price')}</th>
              <th className="num">{t('order.qty')}</th>
              <th className="num">{t('order.total')}</th>
            </tr>
          </thead>
          <tbody>
            {o.items.map((item, i) => (
              <tr key={i}>
                <td>
                  {item.product_id ? <Link to={`/products/${item.product_id}`}>{item.name}</Link> : item.name}
                  {!item.product_id && <span className="muted"> {t('order.deletedProduct')}</span>}
                </td>
                <td className="num">{price(item.price)}</td>
                <td className="num">{item.quantity}</td>
                <td className="num">{price(item.subtotal)}</td>
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <th colSpan={3}>{t('order.total')}</th>
              <th className="num">{price(o.total)}</th>
            </tr>
          </tfoot>
        </table>
      </div>

      <section className="card order-address">
        <h2>{t('order.address')}</h2>
        <p>{o.shipping_address}</p>
      </section>

      {cancelError && <Notice kind="error">{cancelError}</Notice>}
      {o.status === 'pending' && (
        <button type="button" className="btn btn-danger" disabled={cancelling} onClick={cancel}>
          {t('order.cancel')}
        </button>
      )}
    </>
  )
}
