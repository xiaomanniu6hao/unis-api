/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { RefreshCw, Unlock } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import {
  getChannelRuntimeStatus,
  refreshChannelRuntime,
  unlockChannelRuntime,
} from '@/features/dashboard/api'
import type { ChannelRuntimeStatusItem } from '@/features/dashboard/components/channels/types'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

const STATUS_POLL_INTERVAL_MS = 10000

function formatCollectedAt(ts: number): string {
  if (!ts) return '-'
  return dayjs(ts * 1000).format('HH:mm:ss')
}

// 锁状态优先于采集健康度：锁定中的渠道即使样本新鲜也要先提示锁定。
function RuntimeStatusLabel(props: { item: ChannelRuntimeStatusItem }) {
  const { t } = useTranslation()
  if (props.item.locked) {
    return <span className='text-red-600'>{t('Locked')}</span>
  }
  if (props.item.stale) {
    return <span className='text-amber-600'>{t('Stale')}</span>
  }
  return <span className='text-emerald-600'>{t('Healthy')}</span>
}

interface Props {
  // 刷新采集后通知父级重新加载调用分析与趋势数据。
  onRefresh?: () => void
}

export function CurrentRuntimePanel(props: Props) {
  const { t } = useTranslation()
  const [items, setItems] = useState<ChannelRuntimeStatusItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await getChannelRuntimeStatus()
      if (!res?.success) {
        setError(true)
        return
      }
      setItems(res.data?.items || [])
      setError(false)
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const tick = async () => {
      if (cancelled) return
      await load()
    }
    void tick()
    const timer = window.setInterval(() => void tick(), STATUS_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [load])

  const handleRefresh = async () => {
    const res = await refreshChannelRuntime()
    if (res?.success) {
      toast.success(t('Collection triggered'))
      // 采集是异步的：先让父级刷新一次，2 秒后采集落库再一起刷新。
      props.onRefresh?.()
      window.setTimeout(() => {
        void load()
        props.onRefresh?.()
      }, 2000)
      return
    }
    toast.error(res?.message || t('Failed to trigger collection'))
  }

  const handleUnlock = async (channelId: number) => {
    const res = await unlockChannelRuntime(channelId)
    if (res?.success) {
      toast.success(t('Unlocked'))
      void load()
      return
    }
    toast.error(res?.message || t('Failed to unlock'))
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3 sm:px-5'>
        <div>
          <h3 className='text-sm font-semibold'>{t('Current runtime')}</h3>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t('Live Running / Waiting and saturation lock state per channel.')}
          </p>
        </div>
        <Button
          variant='outline'
          size='sm'
          onClick={handleRefresh}
          disabled={loading}
        >
          <RefreshCw className='mr-1 size-3.5' />
          {t('Refresh status')}
        </Button>
      </div>

      <div className='overflow-x-auto'>
        <table className='w-full text-left text-xs'>
          <thead className='bg-muted/50 text-muted-foreground'>
            <tr>
              <th className='px-3 py-2'>{t('Channel')}</th>
              <th className='px-3 py-2'>{t('Models')}</th>
              <th className='px-3 py-2'>Running</th>
              <th className='px-3 py-2'>Waiting</th>
              <th className='px-3 py-2'>{t('Max batch')}</th>
              <th className='px-3 py-2'>KV%</th>
              <th className='px-3 py-2'>{t('Prefix hit%')}</th>
              <th className='px-3 py-2'>{t('Status')}</th>
              <th className='px-3 py-2'>{t('Collected')}</th>
              <th className='px-3 py-2' />
            </tr>
          </thead>
          <tbody>
            {loading && items.length === 0 && (
              <tr>
                <td colSpan={10} className='px-3 py-6'>
                  <Skeleton className='h-5 w-full' />
                </td>
              </tr>
            )}
            {!loading && items.length === 0 && (
              <tr>
                <td
                  colSpan={10}
                  className='text-muted-foreground px-3 py-6 text-center'
                >
                  {error
                    ? t('Failed to load runtime status')
                    : t(
                        'No runtime data yet. Enable collection and wait one interval.'
                      )}
                </td>
              </tr>
            )}
            {items.map((item) => (
              <tr key={item.channel_id} className='border-t'>
                <td className='px-3 py-2'>
                  #{item.channel_id} {item.channel_name}
                </td>
                <td className='max-w-40 truncate px-3 py-2'>
                  {(item.models || []).join(', ')}
                </td>
                <td className='px-3 py-2'>{item.running}</td>
                <td
                  className={cn(
                    'px-3 py-2',
                    item.waiting > 0 && 'text-red-600 dark:text-red-400'
                  )}
                >
                  {item.waiting}
                </td>
                <td className='px-3 py-2'>
                  {item.max_batch || '-'}
                  {item.max_batch_source ? (
                    <span className='text-muted-foreground'>
                      {' '}
                      ({item.max_batch_source})
                    </span>
                  ) : null}
                </td>
                <td className='px-3 py-2'>{item.kv_cache_usage.toFixed(1)}</td>
                <td className='px-3 py-2'>
                  {item.prefix_cache_hit_rate.toFixed(1)}
                </td>
                <td className='px-3 py-2'>
                  <RuntimeStatusLabel item={item} />
                  {item.last_error ? (
                    <span
                      className='text-muted-foreground block max-w-48 truncate'
                      title={item.last_error}
                    >
                      {item.last_error}
                    </span>
                  ) : null}
                </td>
                <td className='px-3 py-2'>
                  {formatCollectedAt(item.last_collected_at)}
                </td>
                <td className='px-3 py-2'>
                  {item.locked ? (
                    <Button
                      variant='ghost'
                      size='sm'
                      onClick={() => void handleUnlock(item.channel_id)}
                    >
                      <Unlock className='mr-1 size-3.5' />
                      {t('Unlock')}
                    </Button>
                  ) : null}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
