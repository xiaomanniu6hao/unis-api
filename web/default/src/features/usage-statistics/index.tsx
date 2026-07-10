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
import { useTranslation } from 'react-i18next'

import { PageTransition } from '@/components/page-transition'

import { getDailyUsageStats, getMonthlyUsageStats } from './api'
import { DistributionTable } from './components/distribution-table'
import { RankUsageStatistics } from './components/rank-usage-statistics'
import { UsageStatsTable } from './components/usage-stats-table'

function defaultDayRange(days: number): [string, string] {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - days)
  return [start.toISOString().slice(0, 10), end.toISOString().slice(0, 10)]
}

function defaultMonthRange(months: number): [string, string] {
  const end = new Date()
  const start = new Date()
  start.setMonth(start.getMonth() - months)
  return [start.toISOString().slice(0, 7), end.toISOString().slice(0, 7)]
}

// 日用量统计页
export function DailyUsageStatistics() {
  const { t } = useTranslation()
  const [start, end] = defaultDayRange(7)
  return (
    <PageTransition className='mx-auto w-full max-w-[1400px] space-y-4 px-4 py-6 sm:px-6'>
      <h1 className='text-xl font-semibold'>{t('Daily Usage Statistics')}</h1>
      <UsageStatsTable
        fetcher={getDailyUsageStats}
        exportBase='/api/usage_statistics'
        initialStartDate={start}
        initialEndDate={end}
        dateLabel={t('Date')}
      />
    </PageTransition>
  )
}

// 月用量统计页
export function MonthlyUsageStatistics() {
  const { t } = useTranslation()
  const [start, end] = defaultMonthRange(6)
  return (
    <PageTransition className='mx-auto w-full max-w-[1400px] space-y-4 px-4 py-6 sm:px-6'>
      <h1 className='text-xl font-semibold'>{t('Monthly Usage Statistics')}</h1>
      <UsageStatsTable
        fetcher={getMonthlyUsageStats}
        exportBase='/api/usage_statistics_monthly'
        initialStartDate={start}
        initialEndDate={end}
        dateLabel={t('Month')}
      />
    </PageTransition>
  )
}

// 输入 tokens 分布页
export function PromptTokensDistribution() {
  const { t } = useTranslation()
  return (
    <DistributionTable
      endpointBase='/api/prompt_tokens_distribution/'
      title={t('Prompt Tokens Distribution')}
    />
  )
}

// 输出 tokens 分布页
export function CompletionTokensDistribution() {
  const { t } = useTranslation()
  return (
    <DistributionTable
      endpointBase='/api/completion_tokens_distribution/'
      title={t('Completion Tokens Distribution')}
    />
  )
}

// 单问题请求次数分布页
export function RequestCountDistribution() {
  const { t } = useTranslation()
  return (
    <DistributionTable
      endpointBase='/api/request_count_distribution/'
      title={t('Request Count Distribution')}
    />
  )
}
