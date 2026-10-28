import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { getChannelResourceStats } from '@/features/dashboard/api'
import type { ChannelResourceItem } from '@/features/dashboard/components/channels/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatCompactNumber, formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

interface Props {
  hours?: number
}

const HOUR_OPTIONS = [24, 24 * 7] as const

type Tier = 'idle' | 'normal' | 'busy' | 'saturated'

interface TierStyle {
  bar: string
  text: string
  badge: string
  label: string
}

const TIER_STYLES: Record<Tier, TierStyle> = {
  idle: {
    bar: 'bg-emerald-500',
    text: 'text-emerald-600 dark:text-emerald-400',
    badge:
      'bg-emerald-500/10 text-emerald-600 ring-1 ring-emerald-500/20 dark:text-emerald-400',
    label: 'Idle',
  },
  normal: {
    bar: 'bg-sky-500',
    text: 'text-sky-600 dark:text-sky-400',
    badge: 'bg-sky-500/10 text-sky-600 ring-1 ring-sky-500/20 dark:text-sky-400',
    label: 'Normal',
  },
  busy: {
    bar: 'bg-amber-500',
    text: 'text-amber-600 dark:text-amber-400',
    badge:
      'bg-amber-500/10 text-amber-600 ring-1 ring-amber-500/20 dark:text-amber-400',
    label: 'Busy',
  },
  saturated: {
    bar: 'bg-red-500',
    text: 'text-red-600 dark:text-red-400',
    badge: 'bg-red-500/10 text-red-600 ring-1 ring-red-500/20 dark:text-red-400',
    label: 'Saturated',
  },
}

function resolveTier(item: ChannelResourceItem): Tier {
  if (item.locked || item.utilization >= 100) return 'saturated'
  if (item.utilization >= 80) return 'busy'
  if (item.utilization >= 50) return 'normal'
  return 'idle'
}

function formatDuration(ms: number): string {
  if (!ms || ms <= 0) return '-'
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)} s`
  return `${(ms / 60000).toFixed(1)} min`
}

export function ChannelResourceAnalytics(props: Props) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const [hours, setHours] = useState<number>(props.hours ?? HOUR_OPTIONS[0])
  const [items, setItems] = useState<ChannelResourceItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false
    const load = async (showLoading: boolean) => {
      if (showLoading) setLoading(true)
      try {
        const res = await getChannelResourceStats(hours)
        if (cancelled) return
        setItems(res.data?.items || [])
        setError(false)
      } catch {
        if (cancelled) return
        setError(true)
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void load(true)
    // 拥挤度是实时数据，10 秒刷新一次；调用情况跟着一起刷新
    const timer = window.setInterval(() => void load(false), 10000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [hours])

  const totalRequests = useMemo(
    () => items.reduce((sum, item) => sum + item.requests, 0),
    [items]
  )

  // 拥挤程度：按利用率从高到低，一眼看出谁快被打满
  const congestionItems = useMemo(
    () =>
      [...items].sort((a, b) => {
        if (b.utilization !== a.utilization) {
          return b.utilization - a.utilization
        }
        return b.waiting - a.waiting
      }),
    [items]
  )

  const compact = (value: number) => formatCompactNumber(value, locale)

  return (
    <div className='space-y-3 sm:space-y-4'>
      {/* 第一块：按渠道调用情况 */}
      <div className='overflow-hidden rounded-lg border'>
        <div className='flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3 sm:px-5'>
          <div>
            <h3 className='text-sm font-semibold'>
              {t('Channel call volume')}
            </h3>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t('Requests, errors and consumption aggregated per channel.')}
            </p>
          </div>
          <div className='bg-muted/60 flex items-center gap-1 rounded-md p-0.5'>
            {HOUR_OPTIONS.map((option) => (
              <button
                key={option}
                type='button'
                onClick={() => setHours(option)}
                className={cn(
                  'rounded px-2 py-1 text-xs transition-colors',
                  hours === option
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:text-foreground'
                )}
              >
                {option === 24 ? t('Last 24h') : t('Last 7d')}
              </button>
            ))}
          </div>
        </div>

        <div className='overflow-x-auto'>
          <table className='w-full min-w-[720px] text-left text-xs'>
            <thead className='bg-muted/40 text-muted-foreground'>
              <tr>
                <th className='px-4 py-2 font-medium'>{t('Channel')}</th>
                <th className='px-4 py-2 font-medium'>{t('Model')}</th>
                <th className='px-4 py-2 text-right font-medium'>
                  {t('Requests')}
                </th>
                <th className='px-4 py-2 font-medium'>{t('Share')}</th>
                <th className='px-4 py-2 text-right font-medium'>
                  {t('Error rate')}
                </th>
                <th className='px-4 py-2 text-right font-medium'>
                  {t('Quota')}
                </th>
                <th className='px-4 py-2 text-right font-medium'>
                  {t('Avg latency')}
                </th>
              </tr>
            </thead>
            <tbody className='divide-border/60 divide-y'>
              {loading && items.length === 0 ? (
                Array.from({ length: 3 }).map((_, idx) => (
                  <tr key={idx}>
                    <td className='px-4 py-2.5' colSpan={7}>
                      <Skeleton className='h-5 w-full' />
                    </td>
                  </tr>
                ))
              ) : items.length === 0 ? (
                <tr>
                  <td
                    className='text-muted-foreground px-4 py-8 text-center'
                    colSpan={7}
                  >
                    {error
                      ? t('Failed to load channel resource data')
                      : t('No channel traffic in this window')}
                  </td>
                </tr>
              ) : (
                items.map((item) => {
                  const share =
                    totalRequests > 0 ? (item.requests / totalRequests) * 100 : 0
                  return (
                    <tr key={item.channel_id} className='hover:bg-muted/30'>
                      <td className='px-4 py-2.5'>
                        <div className='flex items-center gap-2'>
                          <span className='text-muted-foreground font-mono text-[11px]'>
                            #{item.channel_id}
                          </span>
                          <span className='font-medium'>{item.channel_name}</span>
                          {item.status !== 1 && (
                            <span className='bg-muted text-muted-foreground rounded px-1 py-0.5 text-[10px]'>
                              {t('Disabled')}
                            </span>
                          )}
                        </div>
                      </td>
                      <td className='text-muted-foreground max-w-[220px] truncate px-4 py-2.5'>
                        {item.model_names || '-'}
                      </td>
                      <td className='px-4 py-2.5 text-right font-mono tabular-nums'>
                        {compact(item.requests)}
                      </td>
                      <td className='px-4 py-2.5'>
                        <div className='bg-muted h-1.5 w-full min-w-24 overflow-hidden rounded-full'>
                          <div
                            className='bg-sky-500/70 h-full rounded-full'
                            style={{ width: `${Math.max(share, 1)}%` }}
                          />
                        </div>
                        <span className='text-muted-foreground mt-0.5 block font-mono text-[10px]'>
                          {share.toFixed(1)}%
                        </span>
                      </td>
                      <td
                        className={cn(
                          'px-4 py-2.5 text-right font-mono tabular-nums',
                          item.error_rate >= 5 && 'text-red-600 dark:text-red-400'
                        )}
                      >
                        {item.requests + item.error_requests === 0
                          ? '-'
                          : `${item.error_rate.toFixed(1)}%`}
                      </td>
                      <td className='px-4 py-2.5 text-right font-mono tabular-nums'>
                        {formatQuota(item.quota)}
                      </td>
                      <td className='px-4 py-2.5 text-right font-mono tabular-nums'>
                        {formatDuration(item.avg_use_time_ms)}
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* 第二块：渠道拥挤程度 */}
      <div className='overflow-hidden rounded-lg border'>
        <div className='flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3 sm:px-5'>
          <div>
            <h3 className='text-sm font-semibold'>
              {t('Channel congestion')}
            </h3>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t(
                'Running vs max batch from vLLM /metrics. Bar turns red when the channel is saturated.'
              )}
            </p>
          </div>
          <div className='text-muted-foreground flex flex-wrap items-center gap-3 text-[11px]'>
            {(['idle', 'normal', 'busy', 'saturated'] as Tier[]).map((tier) => (
              <span key={tier} className='flex items-center gap-1'>
                <span
                  className={cn('size-2 rounded-full', TIER_STYLES[tier].bar)}
                />
                {t(TIER_STYLES[tier].label)}
              </span>
            ))}
          </div>
        </div>

        <div className='divide-border/60 divide-y'>
          {loading && congestionItems.length === 0 ? (
            <div className='space-y-3 p-4'>
              {Array.from({ length: 3 }).map((_, idx) => (
                <Skeleton key={idx} className='h-12 w-full' />
              ))}
            </div>
          ) : congestionItems.length === 0 ? (
            <div className='text-muted-foreground px-4 py-8 text-center text-xs'>
              {t('No channels')}
            </div>
          ) : (
            congestionItems.map((item) => {
              const tier = resolveTier(item)
              const style = TIER_STYLES[tier]
              const width = item.has_runtime_data
                ? Math.min(item.utilization, 100)
                : 0
              return (
                <div
                  key={item.channel_id}
                  className='px-4 py-3 sm:px-5'
                >
                  <div className='flex flex-wrap items-center justify-between gap-2'>
                    <div className='flex min-w-0 items-center gap-2'>
                      <span className='truncate text-sm font-medium'>
                        {item.channel_name}
                      </span>
                      <span className='text-muted-foreground truncate text-xs'>
                        {item.model_names || '-'}
                      </span>
                      {item.locked && (
                        <span
                          className={cn(
                            'rounded px-1.5 py-0.5 text-[10px] font-medium',
                            style.badge
                          )}
                        >
                          {t('Locked')}
                        </span>
                      )}
                    </div>
                    <div className='flex items-center gap-3 text-xs'>
                      <span className='font-mono tabular-nums'>
                        <span className={cn('font-semibold', style.text)}>
                          {item.running}
                        </span>
                        <span className='text-muted-foreground'>
                          {' / '}
                          {item.max_batch || '-'}
                        </span>
                      </span>
                      <span
                        className={cn(
                          'font-mono tabular-nums',
                          item.waiting > 0
                            ? 'text-red-600 dark:text-red-400'
                            : 'text-muted-foreground'
                        )}
                      >
                        {t('Waiting')} {item.waiting}
                      </span>
                      <span className='text-muted-foreground font-mono tabular-nums'>
                        KV {item.kv_cache_usage.toFixed(0)}%
                      </span>
                      <span
                        className={cn(
                          'rounded px-1.5 py-0.5 text-[10px] font-medium',
                          item.has_runtime_data
                            ? style.badge
                            : 'bg-muted text-muted-foreground'
                        )}
                      >
                        {item.has_runtime_data
                          ? t(style.label)
                          : t('No data')}
                      </span>
                    </div>
                  </div>

                  <div className='bg-muted mt-2 h-2 w-full overflow-hidden rounded-full'>
                    <div
                      className={cn(
                        'h-full rounded-full transition-all',
                        style.bar
                      )}
                      style={{ width: `${width}%` }}
                    />
                  </div>
                  <div className='text-muted-foreground mt-1 flex justify-between text-[10px]'>
                    <span>
                      {t('Utilization')} {item.utilization.toFixed(1)}%
                    </span>
                    <span className='font-mono'>
                      {t('Max batch')} {item.max_batch || '-'}
                      {item.max_batch_source
                        ? ` · ${item.max_batch_source}`
                        : ''}
                    </span>
                  </div>
                </div>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
