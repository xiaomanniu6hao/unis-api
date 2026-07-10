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
import { formatCompactNumber, formatQuota } from '@/lib/format'

import {
  exportStatsCsv,
  getStatsTokens,
  useIsAdmin,
  type StatsQuery,
  type UsageStatisticsItem,
} from '../api'
import { StatsFilterBar } from './stats-filter-bar'
import type { StatsFilterValues } from './stats-filter-bar'
import { StatsPagination } from './stats-pagination'
import { StatsSummaryCards } from './stats-summary-cards'

type UsageStatsTableProps = {
  // 日/月页各自的 fetcher
  fetcher: (
    params: StatsQuery & { p: number; size: number },
    isAdmin: boolean
  ) => Promise<{
    data: {
      items: UsageStatisticsItem[]
      total: number
      page: number
      page_size: number
      summary?: unknown
    }
  }>
  // 导出端点 base，如 /api/usage_statistics
  exportBase: string
  // 初始日期范围
  initialStartDate: string
  initialEndDate: string
  // 是否显示日期列（月用量页日期是月份，仍显示）
  dateLabel: string
}

const PAGE_SIZE = 20

// 日/月用量统计共享表格。由日用量页和月用量页复用。
export function UsageStatsTable({
  fetcher,
  exportBase,
  initialStartDate,
  initialEndDate,
  dateLabel,
}: UsageStatsTableProps) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()

  const [filters, setFilters] = useState<StatsFilterValues>({
    startDate: initialStartDate,
    endDate: initialEndDate,
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
    queryKey: ['usage-stats', appliedFilters, page, isAdmin],
    queryFn: () =>
      fetcher(
        {
          start_date: appliedFilters.startDate,
          end_date: appliedFilters.endDate,
          token_id: appliedFilters.tokenId || undefined,
          model_name: appliedFilters.modelName || undefined,
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
      startDate: initialStartDate,
      endDate: initialEndDate,
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
        exportBase,
        {
          start_date: appliedFilters.startDate,
          end_date: appliedFilters.endDate,
          token_id: appliedFilters.tokenId || undefined,
          model_name: appliedFilters.modelName || undefined,
        },
        isAdmin,
        `${exportBase.replace('/api/', '')}.csv`
      )
    } catch {
      // axios 拦截器已 toast
    }
  }

  return (
    <div className='space-y-4'>
      <StatsFilterBar
        value={filters}
        onChange={setFilters}
        onSearch={handleSearch}
        onReset={handleReset}
        onExport={handleExport}
        tokens={tokensQuery.data ?? []}
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
            <Table className='min-w-[900px]'>
              <TableHeader>
                <TableRow className='bg-muted/40 hover:bg-muted/40'>
                  <TableHead className='h-9 px-3 text-xs'>
                    {dateLabel}
                  </TableHead>
                  <TableHead className='h-9 px-3 text-xs'>
                    {t('Token Name')}
                  </TableHead>
                  <TableHead className='h-9 px-3 text-xs'>
                    {t('Model')}
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
                    {t('Quota')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((item) => (
                  <TableRow key={item.id} className='hover:bg-muted/30'>
                    <TableCell className='px-3 py-2 text-xs whitespace-nowrap'>
                      {item.date}
                    </TableCell>
                    <TableCell className='px-3 py-2 text-xs'>
                      {item.token_name || t('Unknown')}
                    </TableCell>
                    <TableCell className='text-muted-foreground px-3 py-2 text-xs'>
                      {item.model_name || '-'}
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
                      {formatQuota(item.total_quota)}
                    </TableCell>
                  </TableRow>
                ))}
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
  )
}
