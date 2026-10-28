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
import { useQuery } from '@tanstack/react-query'
import { VChart } from '@visactor/react-vchart'
import { KeyRound, Loader2, WalletCards, Hash, Activity } from 'lucide-react'
import { useMemo, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { getTokenQuotaDataByTokens } from '@/features/dashboard/api'
import {
  TIME_GRANULARITY_OPTIONS,
  TIME_RANGE_PRESETS,
} from '@/features/dashboard/constants'
import {
  buildQueryParams,
  getDefaultDays,
  saveGranularity,
  processTokenChartData,
} from '@/features/dashboard/lib'
import type {
  DashboardFilters,
  ProcessedTokenChartData,
  UserChartsFilters,
  UserMetric,
} from '@/features/dashboard/types'
import {
  getRollingDateRange,
  computeTimeRange,
  type TimeGranularity,
} from '@/lib/time'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'

const TOKEN_CHARTS: {
  value: string
  labelKey: string
  specKey: keyof ProcessedTokenChartData
}[] = [
  {
    value: 'rank',
    labelKey: 'Token Consumption Ranking',
    specKey: 'spec_token_rank',
  },
  {
    value: 'trend',
    labelKey: 'Token Consumption Trend',
    specKey: 'spec_token_trend',
  },
]

const TOKEN_METRIC_OPTIONS = [
  { value: 'quota', labelKey: 'By quota', icon: WalletCards },
  { value: 'tokens', labelKey: 'By tokens', icon: Hash },
  { value: 'requests', labelKey: 'By requests', icon: Activity },
] as const

const TOP_TOKEN_LIMIT_OPTIONS = [5, 10, 20, 50]

interface TokenChartsProps {
  filters: UserChartsFilters
  onFiltersChange: (filters: UserChartsFilters) => void
  // 看板级筛选（时间范围、排除令牌等），由筛选对话框维护
  dashboardFilters?: DashboardFilters
}

export function TokenCharts(props: TokenChartsProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()

  // 与用户统计一致：选择状态由看板父级持有，切换子区块后保留。
  const timeGranularity = props.filters.timeGranularity
  const selectedRange = props.filters.selectedRange
  const topUserLimit = props.filters.topUserLimit
  const metric = props.filters.metric
  const onFiltersChange = props.onFiltersChange

  const timeRange = useMemo(() => {
    const { start, end } = getRollingDateRange(selectedRange)
    return {
      start_timestamp: Math.floor(start.getTime() / 1000),
      end_timestamp: Math.floor(end.getTime() / 1000),
    }
  }, [selectedRange])

  // 筛选对话框选了自定义时间范围时优先使用，否则用滚动窗口
  const queryParams = useMemo(
    () =>
      buildQueryParams(
        props.dashboardFilters?.start_timestamp ||
          props.dashboardFilters?.end_timestamp
          ? computeTimeRange(
              selectedRange,
              props.dashboardFilters?.start_timestamp,
              props.dashboardFilters?.end_timestamp
            )
          : timeRange,
        props.dashboardFilters
      ),
    [props.dashboardFilters, selectedRange, timeRange]
  )

  const handleRangeChange = useCallback(
    (days: number) => {
      onFiltersChange({ ...props.filters, selectedRange: days })
    },
    [onFiltersChange, props.filters]
  )

  const handleGranularityChange = useCallback(
    (g: TimeGranularity) => {
      saveGranularity(g)
      onFiltersChange({
        ...props.filters,
        timeGranularity: g,
        selectedRange: getDefaultDays(g),
      })
    },
    [onFiltersChange, props.filters]
  )

  const handleTopTokenLimitChange = useCallback(
    (limit: number) => {
      onFiltersChange({ ...props.filters, topUserLimit: limit })
    },
    [onFiltersChange, props.filters]
  )

  const handleMetricChange = useCallback(
    (m: UserMetric) => {
      onFiltersChange({ ...props.filters, metric: m })
    },
    [onFiltersChange, props.filters]
  )

  const { data: tokenData, isLoading } = useQuery({
    queryKey: ['dashboard', 'token-quota', queryParams],
    queryFn: () => getTokenQuotaDataByTokens(queryParams),
    select: (res) => (res.success ? res.data : []),
    staleTime: 60_000,
  })

  const chartData = useMemo(
    () =>
      processTokenChartData(
        isLoading ? [] : (tokenData ?? []),
        timeGranularity,
        t,
        topUserLimit,
        metric
      ),
    [tokenData, isLoading, timeGranularity, t, topUserLimit, metric]
  )

  return (
    <div className='space-y-3'>
      <div className='flex items-center gap-1.5 overflow-x-auto pb-1 sm:gap-2'>
        <Tabs
          value={metric}
          onValueChange={(value) => handleMetricChange(value as UserMetric)}
          className='shrink-0'
        >
          <TabsList aria-label={t('Value metric')}>
            {TOKEN_METRIC_OPTIONS.map((option) => {
              const Icon = option.icon
              return (
                <TabsTrigger
                  key={option.value}
                  value={option.value}
                  className='gap-1.5 px-2.5 text-xs'
                >
                  <Icon data-icon='inline-start' aria-hidden='true' />
                  {t(option.labelKey)}
                </TabsTrigger>
              )
            })}
          </TabsList>
        </Tabs>

        <Tabs
          value={String(selectedRange)}
          onValueChange={(value) => handleRangeChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            {TIME_RANGE_PRESETS.map((preset) => (
              <TabsTrigger
                key={preset.days}
                value={String(preset.days)}
                className='px-2.5 text-xs'
              >
                {t(preset.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={timeGranularity}
          onValueChange={(value) =>
            handleGranularityChange(value as TimeGranularity)
          }
          className='shrink-0'
        >
          <TabsList>
            {TIME_GRANULARITY_OPTIONS.map((opt) => (
              <TabsTrigger
                key={opt.value}
                value={opt.value}
                className='px-2.5 text-xs'
              >
                {t(opt.label)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Tabs
          value={String(topUserLimit)}
          onValueChange={(value) => handleTopTokenLimitChange(Number(value))}
          className='shrink-0'
        >
          <TabsList>
            <span className='text-muted-foreground px-2 text-xs font-medium whitespace-nowrap'>
              {t('Top Tokens')}
            </span>
            {TOP_TOKEN_LIMIT_OPTIONS.map((limit) => (
              <TabsTrigger
                key={limit}
                value={String(limit)}
                className='px-2.5 text-xs'
              >
                {t('Top {{count}}', { count: limit })}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        {isLoading && (
          <Loader2 className='text-muted-foreground size-4 animate-spin' />
        )}
      </div>

      <div className='grid gap-3'>
        {TOKEN_CHARTS.map((chart) => {
          const spec = chartData[chart.specKey]

          return (
            <div
              key={chart.value}
              className='overflow-hidden rounded-lg border'
            >
              <div className='flex w-full items-center gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
                <KeyRound className='text-muted-foreground/60 size-4' />
                <div className='text-sm font-semibold'>{t(chart.labelKey)}</div>
              </div>

              <div className='h-[300px] p-1.5 sm:h-96 sm:p-2'>
                {isLoading ? (
                  <Skeleton className='h-full w-full' />
                ) : (
                  themeReady &&
                  spec && (
                    <VChart
                      key={`token-${chart.value}-${topUserLimit}-${metric}-${resolvedTheme}`}
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
    </div>
  )
}