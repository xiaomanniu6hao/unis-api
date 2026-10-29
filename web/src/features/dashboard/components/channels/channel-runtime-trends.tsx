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
import { VChart } from '@visactor/react-vchart'
import { Activity } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useTheme } from '@/context/theme-provider'
import { getChannelRuntimeSeries } from '@/features/dashboard/api'
import type {
  ChannelRuntimeChannelItem,
  ChannelRuntimeSeriesItem,
} from '@/features/dashboard/components/channels/types'
import { useVChartThemeReady } from '@/features/dashboard/hooks/use-vchart-theme'
import {
  buildChannelConcurrencySpec,
  buildChannelThroughputSpec,
} from '@/features/dashboard/lib/channel-charts'
import {
  channelRangeParams,
  channelRuntimeBucketSeconds,
  type ChannelTimeRange,
} from '@/features/dashboard/lib/channel-range'
import { cn } from '@/lib/utils'
import { VCHART_OPTION } from '@/lib/vchart'

type TrendTab = 'concurrency' | 'prefill' | 'decode'

const TREND_TABS: Array<{ value: TrendTab; labelKey: string }> = [
  { value: 'concurrency', labelKey: 'Concurrency' },
  { value: 'prefill', labelKey: 'Prefill performance' },
  { value: 'decode', labelKey: 'Decode performance' },
]

interface Props {
  range: ChannelTimeRange
  // 刷新采集后由父级递增，触发重新拉取趋势数据。
  refreshKey?: number
}

// 渠道卡片按时间窗内的平均利用率排序，最忙的排在最前面。
function averageUtilization(points: ChannelRuntimeSeriesItem[]): number {
  const ratios = points
    .filter((point) => point.max_batch > 0)
    .map((point) => point.avg_running / point.max_batch)
  if (ratios.length === 0) return 0
  return ratios.reduce((sum, ratio) => sum + ratio, 0) / ratios.length
}

export function ChannelRuntimeTrends(props: Props) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const themeReady = useVChartThemeReady()
  const [items, setItems] = useState<ChannelRuntimeSeriesItem[]>([])
  const [channels, setChannels] = useState<ChannelRuntimeChannelItem[]>([])
  const [loading, setLoading] = useState(true)
  const [bucketSeconds, setBucketSeconds] = useState(
    channelRuntimeBucketSeconds(props.range)
  )
  const [activeTab, setActiveTab] = useState<TrendTab>('concurrency')

  const params = useMemo(
    () => ({
      ...channelRangeParams(props.range),
      bucket_seconds: channelRuntimeBucketSeconds(props.range),
    }),
    [props.range]
  )

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    void getChannelRuntimeSeries(params)
      .then((res) => {
        if (cancelled) return
        if (!res?.success) {
          toast.error(res?.message || t('Failed to load runtime status'))
          setItems([])
          setChannels([])
          return
        }
        setItems(res.data?.items || [])
        setChannels(res.data?.channels || [])
        setBucketSeconds(res.data?.bucket_seconds || params.bucket_seconds)
      })
      .catch(() => {
        if (cancelled) return
        toast.error(t('Failed to load runtime status'))
        setItems([])
        setChannels([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [params, t, props.refreshKey])

  const byChannel = useMemo(() => {
    const grouped = new Map<number, ChannelRuntimeSeriesItem[]>()
    items.forEach((item) => {
      const list = grouped.get(item.channel_id) || []
      list.push(item)
      grouped.set(item.channel_id, list)
    })
    return grouped
  }, [items])

  // 后端返回的渠道列表覆盖了「有渠道但窗口内没有样本」的情况；
  // 万一列表缺失，也把趋势里出现过的渠道补上。
  const orderedChannels = useMemo(() => {
    const seen = new Set<number>()
    const all: ChannelRuntimeChannelItem[] = []
    channels.forEach((channel) => {
      seen.add(channel.id)
      all.push(channel)
    })
    items.forEach((item) => {
      if (seen.has(item.channel_id)) return
      seen.add(item.channel_id)
      all.push({
        id: item.channel_id,
        name: item.channel_name || `#${item.channel_id}`,
        models: item.model_names
          ? item.model_names
              .split(',')
              .map((name) => name.trim())
              .filter(Boolean)
          : [],
        status: 1,
      })
    })
    return all.sort((a, b) => {
      const diff =
        averageUtilization(byChannel.get(b.id) || []) -
        averageUtilization(byChannel.get(a.id) || [])
      return diff !== 0 ? diff : a.id - b.id
    })
  }, [channels, items, byChannel])

  if (loading && items.length === 0) {
    return (
      <div className='space-y-3'>
        <Skeleton className='h-72 w-full' />
        <Skeleton className='h-72 w-full' />
      </div>
    )
  }

  if (orderedChannels.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border px-4 py-8 text-center text-xs'>
        {t('No channels')}
      </div>
    )
  }

  return (
    <div className='space-y-3'>
      {orderedChannels.map((channel) => {
        const points = byChannel.get(channel.id) || []
        const latest = points.at(-1)
        const spec =
          activeTab === 'concurrency'
            ? buildChannelConcurrencySpec(points, bucketSeconds, t)
            : buildChannelThroughputSpec(
                points,
                bucketSeconds,
                activeTab === 'prefill' ? 'prompt' : 'generation',
                t
              )
        return (
          <div key={channel.id} className='overflow-hidden rounded-lg border'>
            <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
              <div className='flex min-w-0 flex-wrap items-center gap-2'>
                <IconBadge tone='chart-1' size='sm'>
                  <Activity />
                </IconBadge>
                <span className='truncate text-sm font-semibold'>
                  {channel.name || `#${channel.id}`}
                </span>
                <span className='text-muted-foreground truncate text-xs'>
                  {channel.models.join(', ') || '-'}
                </span>
                {channel.status !== 1 && (
                  <span className='bg-muted text-muted-foreground rounded px-1.5 py-0.5 text-[10px] font-medium'>
                    {t('Disabled')}
                  </span>
                )}
                <span className='text-muted-foreground font-mono text-[11px]'>
                  {t('Max batch')} {latest?.max_batch || '-'}
                  {latest ? ` · ${t('Peak')} ${latest.max_running}` : ''}
                </span>
              </div>

              <div className='bg-muted/60 inline-flex h-7 w-full overflow-x-auto rounded-lg border p-0.5 sm:h-8 sm:w-auto'>
                {TREND_TABS.map((tab) => (
                  <button
                    key={tab.value}
                    type='button'
                    onClick={() => setActiveTab(tab.value)}
                    className={cn(
                      'shrink-0 rounded-md px-3 text-xs font-medium transition-colors',
                      activeTab === tab.value
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    {t(tab.labelKey)}
                  </button>
                ))}
              </div>
            </div>

            <div className='h-[240px] p-1.5 sm:h-80 sm:p-2'>
              {points.length === 0 ? (
                <div className='text-muted-foreground flex h-full items-center justify-center text-xs'>
                  {t('No data available')}
                </div>
              ) : (
                themeReady && (
                  <VChart
                    key={`${channel.id}-${activeTab}`}
                    spec={{
                      ...spec,
                      theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                      background: 'transparent',
                    }}
                    option={VCHART_OPTION}
                  />
                )
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
