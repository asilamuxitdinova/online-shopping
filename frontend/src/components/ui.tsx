import { useState, type CSSProperties, type ReactNode } from 'react'
import { useI18n } from '../i18n/I18nProvider'
import type { OrderStatus } from '../api/types'

export function Notice({ kind = 'info', children }: { kind?: 'info' | 'success' | 'error'; children: ReactNode }) {
  return (
    <div className={`notice notice-${kind}`} role={kind === 'error' ? 'alert' : 'status'}>
      {children}
    </div>
  )
}

/** Error notice with a retry button, for failed page loads. */
export function LoadError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t, error: message } = useI18n()
  return (
    <Notice kind="error">
      <span>{message(error)}</span>{' '}
      <button type="button" className="btn btn-sm" onClick={onRetry}>
        {t('common.retry')}
      </button>
    </Notice>
  )
}

export function Loading() {
  const { t } = useI18n()
  return (
    <div className="loading" role="status">
      <span className="spinner" aria-hidden="true" />
      {t('common.loading')}
    </div>
  )
}

export function StockLabel({ stock }: { stock: number }) {
  const { t } = useI18n()
  if (stock <= 0) return <span className="stock stock-out">{t('product.outOfStock')}</span>
  if (stock <= 5) return <span className="stock stock-low">{t('product.fewLeft', { count: stock })}</span>
  return <span className="stock stock-in">{t('product.inStock')}</span>
}

export function StatusBadge({ status }: { status: OrderStatus }) {
  const { t } = useI18n()
  return <span className={`badge badge-${status}`}>{t(`status.${status}`)}</span>
}

/** Product photo, or a coloured tile with the product's initial when there is none. */
export function ProductImage({ id, name, url, className = '' }: { id: number; name: string; url: string | null; className?: string }) {
  const [failed, setFailed] = useState(false)
  if (url && !failed) {
    return <img className={`product-image ${className}`} src={url} alt={name} loading="lazy" onError={() => setFailed(true)} />
  }
  const hue = (id * 47) % 360
  return (
    <div className={`product-image product-placeholder ${className}`} style={{ '--hue': hue } as CSSProperties} aria-hidden="true">
      {name.trim().charAt(0).toUpperCase()}
    </div>
  )
}

export function QuantityStepper({
  value,
  max,
  onChange,
  disabled,
}: {
  value: number
  max: number
  onChange: (value: number) => void
  disabled?: boolean
}) {
  const { t } = useI18n()
  const [draft, setDraft] = useState<string | null>(null)
  const clamp = (n: number) => Math.min(Math.max(1, Math.floor(n)), Math.max(1, max))

  const commit = () => {
    if (draft === null) return
    const n = Number(draft)
    setDraft(null)
    if (Number.isFinite(n) && clamp(n) !== value) onChange(clamp(n))
  }

  return (
    <div className="stepper">
      <button type="button" aria-label={t('cart.decrease')} disabled={disabled || value <= 1} onClick={() => onChange(value - 1)}>
        −
      </button>
      <input
        type="number"
        inputMode="numeric"
        min={1}
        max={max}
        aria-label={t('product.quantity')}
        value={draft ?? value}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={commit}
        onKeyDown={(e) => e.key === 'Enter' && commit()}
      />
      <button type="button" aria-label={t('cart.increase')} disabled={disabled || value >= max} onClick={() => onChange(value + 1)}>
        +
      </button>
    </div>
  )
}

export function Pagination({ page, pages, onChange }: { page: number; pages: number; onChange: (page: number) => void }) {
  const { t } = useI18n()
  if (pages <= 1) return null
  return (
    <nav className="pagination" aria-label="pagination">
      <button type="button" className="btn" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        ← {t('common.previous')}
      </button>
      <span className="muted">{t('common.page', { page, pages })}</span>
      <button type="button" className="btn" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        {t('common.next')} →
      </button>
    </nav>
  )
}
