import { useEffect, useState, type FormEvent } from 'react'
import { useSearchParams } from 'react-router'
import { api } from '../api/api'
import { ProductCard } from '../components/ProductCard'
import { LoadError, Loading, Pagination } from '../components/ui'
import { useAsync, useTitle } from '../hooks'
import { useI18n } from '../i18n/I18nProvider'

const PAGE_SIZE = 12
const sorts = ['newest', 'price_asc', 'price_desc', 'name'] as const

export function CatalogPage() {
  const { t, category: categoryLabel } = useI18n()
  const [params, setParams] = useSearchParams()

  const search = params.get('search') ?? ''
  const category = params.get('category') ?? ''
  const minPrice = params.get('minPrice') ?? ''
  const maxPrice = params.get('maxPrice') ?? ''
  const sort = params.get('sort') ?? 'newest'
  const page = Math.max(1, Number(params.get('page')) || 1)

  const products = useAsync(
    () => api.products({ search, category, minPrice, maxPrice, sort, page, limit: PAGE_SIZE }),
    [search, category, minPrice, maxPrice, sort, page],
  )
  const categories = useAsync(() => api.categories(), [])

  const heading = search ? t('catalog.searchFor', { query: search }) : category ? categoryLabel(category) : t('catalog.title')
  useTitle(heading)

  /** Updates query parameters; any filter change goes back to page 1. */
  const update = (changes: Record<string, string | null>) => {
    const next = new URLSearchParams(params)
    for (const [key, value] of Object.entries(changes)) {
      if (value) next.set(key, value)
      else next.delete(key)
    }
    if (!('page' in changes)) next.delete('page')
    setParams(next)
  }

  const [priceDraft, setPriceDraft] = useState({ min: minPrice, max: maxPrice })
  useEffect(() => {
    setPriceDraft({ min: minPrice, max: maxPrice })
  }, [minPrice, maxPrice])

  const applyPrice = (e: FormEvent) => {
    e.preventDefault()
    update({ minPrice: priceDraft.min, maxPrice: priceDraft.max })
  }

  const hasFilters = Boolean(search || category || minPrice || maxPrice)
  const data = products.data

  return (
    <div className="catalog">
      <aside className="card filters" aria-label={t('catalog.filters')}>
        <h2 className="filters-title">{t('catalog.categories')}</h2>
        <ul className="category-list">
          <li>
            <button type="button" aria-pressed={!category} onClick={() => update({ category: null })}>
              {t('catalog.allCategories')}
            </button>
          </li>
          {categories.data?.map((c) => (
            <li key={c.category}>
              <button type="button" aria-pressed={c.category === category} onClick={() => update({ category: c.category })}>
                {categoryLabel(c.category)}
                <span className="muted">{c.count}</span>
              </button>
            </li>
          ))}
        </ul>

        <h2 className="filters-title">
          {t('catalog.price')}, {t('currency')}
        </h2>
        <form className="price-filter" onSubmit={applyPrice}>
          <input
            type="number"
            min={0}
            step={1000}
            inputMode="numeric"
            placeholder={t('catalog.min')}
            aria-label={`${t('catalog.price')} ${t('catalog.min')}`}
            value={priceDraft.min}
            onChange={(e) => setPriceDraft((d) => ({ ...d, min: e.target.value }))}
          />
          <span aria-hidden="true">–</span>
          <input
            type="number"
            min={0}
            step={1000}
            inputMode="numeric"
            placeholder={t('catalog.max')}
            aria-label={`${t('catalog.price')} ${t('catalog.max')}`}
            value={priceDraft.max}
            onChange={(e) => setPriceDraft((d) => ({ ...d, max: e.target.value }))}
          />
          <button type="submit" className="btn btn-sm">
            {t('catalog.apply')}
          </button>
        </form>

        {hasFilters && (
          <button type="button" className="link-button" onClick={() => setParams(sort !== 'newest' ? { sort } : {})}>
            {t('catalog.reset')}
          </button>
        )}
      </aside>

      <section className="catalog-results">
        <div className="toolbar">
          <div>
            <h1>{heading}</h1>
            {data && <p className="muted">{t('catalog.results', { count: data.total })}</p>}
          </div>
          <label className="sort">
            <span>{t('catalog.sort')}</span>
            <select value={sort} onChange={(e) => update({ sort: e.target.value === 'newest' ? null : e.target.value })}>
              {sorts.map((s) => (
                <option key={s} value={s}>
                  {t(`sort.${s}`)}
                </option>
              ))}
            </select>
          </label>
        </div>

        {products.error ? (
          <LoadError error={products.error} onRetry={products.reload} />
        ) : !data ? (
          <Loading />
        ) : data.items.length === 0 ? (
          <div className="empty card">
            <p>{t('catalog.empty')}</p>
            {hasFilters && (
              <button type="button" className="btn" onClick={() => setParams({})}>
                {t('catalog.reset')}
              </button>
            )}
          </div>
        ) : (
          <div className={`product-grid ${products.loading ? 'is-loading' : ''}`}>
            {data.items.map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        )}

        {data && <Pagination page={data.page} pages={data.pages} onChange={(p) => update({ page: p > 1 ? String(p) : null })} />}
      </section>
    </div>
  )
}
