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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { ErrorState } from '@/components/error-state'
import { Skeleton } from '@/components/ui/skeleton'
import dayjs from '@/lib/dayjs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatCompactNumber, formatQuota } from '@/lib/format'

import {
  exportStatsCsv,
  getRankUsageStats,
  getStatsTokens,
  useIsAdmin,
  type UsageStatisticsItem,
} from '../api'
import { StatsFilterBar } from './stats-filter-bar'
import type { StatsFilterValues } from './stats-filter-bar'
import { StatsPagination } from './stats-pagination'
import { StatsSummaryCards } from './stats-summary-cards'

const PAGE_SIZE = 20

// 默认取当月 1 日到今天（YYYY-MM-DD，本地时区）。
function defaultMonthStartRange(): [string, string] {
  // 用 dayjs 本地时区格式化，避免 toISOString() 转 UTC 导致跨月/跨日偏移
  const now = dayjs()
  const start = now.startOf('month')
  return [start.format('YYYY-MM-DD'), now.format('YYYY-MM-DD')]
}

// 用量排序统计页：按 token 聚合，展示请求/token/额度/缓存命中率/min/max/问题数。
export function RankUsageStatistics() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const [startInit, endInit] = defaultMonthStartRange()

  const [filters, setFilters] = useState<StatsFilterValues>({
    startDate: startInit,
    endDate: endInit,
    tokenId: 0,
    modelName: '',
    groupBy: 'prefix',
  })
  const [page, setPage] = useState(1)
  const [appliedFilters, setAppliedFilters] = useState(filters)

  const tokensQuery = useQuery({
    queryKey: ['stats-tokens', isAdmin],
    queryFn: () => getStatsTokens(),
    staleTime: 5 * 60 * 1000,
  })

  const statsQuery = useQuery({
    queryKey: ['rank-stats', appliedFilters, page, isAdmin],
    queryFn: () =>
      getRankUsageStats(
        {
          start_date: appliedFilters.startDate,
          end_date: appliedFilters.endDate,
          model_name: appliedFilters.modelName || undefined,
          group_by: appliedFilters.groupBy,
          p: page,
          size: PAGE_SIZE,
        },
        isAdmin
      ),
    staleTime: 30 * 1000,
  })

  const data = statsQuery.data?.data
  const items = data?.items ?? []
  const total = data?.total ?? 0
  const summary = (data?.summary ?? null) as
    | import('../api').UsageSummary
    | null

  const handleSearch = () => {
    setAppliedFilters(filters)
    setPage(1)
  }
  const handleReset = () => {
    const reset: StatsFilterValues = {
      startDate: startInit,
      endDate: endInit,
      tokenId: 0,
      modelName: '',
      groupBy: 'prefix',
    }
    setFilters(reset)
    setAppliedFilters(reset)
    setPage(1)
  }
  const handleExport = async () => {
    try {
      await exportStatsCsv(
        '/api/usage_statistics_rank',
        {
          start_date: appliedFilters.startDate,
          end_date: appliedFilters.endDate,
          model_name: appliedFilters.modelName || undefined,
          group_by: appliedFilters.groupBy,
        },
        isAdmin,
        'usage_rank.csv'
      )
    } catch {
      // axios 拦截器已 toast
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage Ranking')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <StatsFilterBar
            value={filters}
            onChange={setFilters}
            onSearch={handleSearch}
            onReset={handleReset}
            onExport={handleExport}
            tokens={tokensQuery.data ?? []}
            showGroupBy
            showTokenFilter={false}
          />

          <StatsSummaryCards summary={summary} />

          <div className='bg-card overflow-hidden rounded-lg border'>
            {statsQuery.isLoading ? (
              <div className='space-y-2 p-4'>
                {Array.from({ length: 6 }).map((_, i) => (
                  <Skeleton key={i} className='h-9 w-full rounded-md' />
                ))}
              </div>
            ) : statsQuery.isError ? (
              <ErrorState
                title={t('Failed to load statistics')}
                description={
                  statsQuery.error instanceof Error
                    ? statsQuery.error.message
                    : undefined
                }
                onRetry={() => void statsQuery.refetch()}
                className='min-h-[260px]'
              />
            ) : items.length === 0 ? (
              <div className='text-muted-foreground px-4 py-10 text-center text-sm'>
                {t('No data')}
              </div>
            ) : (
              <div className='overflow-x-auto'>
                <Table className='min-w-[1200px]'>
                <TableHeader>
                  <TableRow className='bg-muted/40 hover:bg-muted/40'>
                    <TableHead className='h-9 px-3 text-xs'>
                      {t('Token Name')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Questions')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Requests')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Prompt')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Completion')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Total Tokens')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Cache')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Cache Hit')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Avg Prompt')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Avg Completion')}
                    </TableHead>
                    <TableHead className='h-9 px-3 text-right text-xs'>
                      {t('Quota')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {items.map((item) => {
                    const cacheHit =
                      item.prompt_tokens > 0
                        ? (item.prompt_tokens_cache / item.prompt_tokens) *
                          100
                        : 0
                    return (
                      <TableRow key={item.id} className='hover:bg-muted/30'>
                        <TableCell className='px-3 py-2 text-xs font-medium'>
                          {item.token_name || t('Unknown')}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(item.question_count)}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(item.total_requests)}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(item.prompt_tokens)}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(item.completion_tokens)}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs font-medium tabular-nums'>
                          {formatCompactNumber(item.total_tokens)}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(item.prompt_tokens_cache)}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {cacheHit.toFixed(1)}%
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(Math.round(item.avg_prompt_tokens))}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatCompactNumber(
                            Math.round(item.avg_completion_tokens)
                          )}
                        </TableCell>
                        <TableCell className='px-3 py-2 text-right text-xs tabular-nums'>
                          {formatQuota(item.total_quota)}
                        </TableCell>
                      </TableRow>
                    )
                  })}
                </TableBody>
              </Table>
            </div>
          )}
        </div>

        <StatsPagination
          page={page}
          pageSize={PAGE_SIZE}
          total={total}
          onPageChange={setPage}
        />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
