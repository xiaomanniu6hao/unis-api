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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { ErrorState } from '@/components/error-state'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatCompactNumber } from '@/lib/format'

import {
  exportStatsCsv,
  getDistributionStats,
  getStatsTokens,
  useIsAdmin,
  type DistributionItem,
} from '../api'
import { StatsFilterBar } from './stats-filter-bar'
import type { StatsFilterValues } from './stats-filter-bar'
import { StatsPagination } from './stats-pagination'

const PAGE_SIZE = 20

function defaultMonthStartRange(): [string, string] {
  const now = new Date()
  const start = new Date(now.getFullYear(), now.getMonth(), 1)
  return [start.toISOString().slice(0, 10), now.toISOString().slice(0, 10)]
}

type DistributionTableProps = {
  // 完整端点前缀，如 /api/prompt_tokens_distribution/
  endpointBase: string
  title: string
}

// 分布统计共享表格：行=token，列=动态桶（range_groups），单元格=count（可切换占比）。
// prompt/completion/request_count 三个分布页复用此组件。
export function DistributionTable({
  endpointBase,
  title,
}: DistributionTableProps) {
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
  const [showPercent, setShowPercent] = useState(false)

  const tokensQuery = useQuery({
    queryKey: ['stats-tokens', isAdmin],
    queryFn: () => getStatsTokens(),
    staleTime: 5 * 60 * 1000,
  })

  const statsQuery = useQuery({
    queryKey: ['distribution', endpointBase, appliedFilters, page, isAdmin],
    queryFn: () =>
      getDistributionStats(
        endpointBase,
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
  const rangeGroups = data?.range_groups ?? []
  const total = data?.total ?? 0

  // 按 token 聚合：每个 token 在各桶的 count 与后端返回的 percent、总计。
  const { rows, tokenOrder } = useMemo(() => {
    const map = new Map<string, Map<string, { count: number; percent: number }>>()
    const order: string[] = []
    for (const item of items as DistributionItem[]) {
      let bucket = map.get(item.token_name)
      if (!bucket) {
        bucket = new Map()
        map.set(item.token_name, bucket)
        order.push(item.token_name)
      }
      const prev = bucket.get(item.range_grp)
      bucket.set(item.range_grp, {
        count: (prev?.count ?? 0) + item.count,
        percent: item.percent,
      })
    }
    return {
      rows: map,
      tokenOrder: order,
    }
  }, [items])

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
        endpointBase.replace(/\/$/, ''),
        {
          start_date: appliedFilters.startDate,
          end_date: appliedFilters.endDate,
          model_name: appliedFilters.modelName || undefined,
          group_by: appliedFilters.groupBy,
        },
        isAdmin,
        `${endpointBase.replace('/api/', '').replace(/\/$/, '')}.csv`
      )
    } catch {
      // axios 拦截器已 toast
    }
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{title}</SectionPageLayout.Title>
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
            showPercentToggle
            percent={showPercent}
            onTogglePercent={setShowPercent}
          />

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
            ) : tokenOrder.length === 0 ? (
              <div className='text-muted-foreground px-4 py-10 text-center text-sm'>
                {t('No data')}
              </div>
            ) : (
              <div className='overflow-x-auto'>
                <Table className='min-w-[800px]'>
                  <TableHeader>
                    <TableRow className='bg-muted/40 hover:bg-muted/40'>
                      <TableHead className='h-9 px-3 text-xs'>
                        {t('Token Name')}
                      </TableHead>
                      {rangeGroups.map((g) => (
                        <TableHead
                          key={g}
                          className='h-9 px-3 text-right text-xs whitespace-nowrap'
                        >
                          {g}
                        </TableHead>
                      ))}
                      <TableHead className='h-9 px-3 text-right text-xs'>
                        {t('Total')}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tokenOrder.map((name) => {
                      const bucket = rows.get(name)!
                      let sum = 0
                      return (
                        <TableRow key={name} className='hover:bg-muted/30'>
                          <TableCell className='px-3 py-2 text-xs font-medium'>
                            {name || t('Unknown')}
                          </TableCell>
                          {rangeGroups.map((g) => {
                            const cell = bucket.get(g)
                            const cnt = cell?.count ?? 0
                            const pct = cell?.percent ?? 0
                            sum += cnt
                            return (
                              <TableCell
                                key={g}
                                className='px-3 py-2 text-right text-xs tabular-nums'
                              >
                                <span className='inline-flex items-baseline gap-1'>
                                  {formatCompactNumber(cnt)}
                                  {showPercent && (
                                    <span className='text-muted-foreground'>
                                      ({pct.toFixed(1)}%)
                                    </span>
                                  )}
                                </span>
                              </TableCell>
                            )
                          })}
                          <TableCell className='px-3 py-2 text-right text-xs font-medium tabular-nums'>
                            {formatCompactNumber(sum)}
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
