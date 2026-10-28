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

import { SectionPageLayout } from '@/components/layout'
import dayjs from '@/lib/dayjs'

import { getDailyUsageStats, getMonthlyUsageStats } from './api'
import { DistributionTable } from './components/distribution-table'
import { RankUsageStatistics } from './components/rank-usage-statistics'
import { UsageStatsTable } from './components/usage-stats-table'

export { RankUsageStatistics }

// 日用量统计页
export function DailyUsageStatistics() {
  const { t } = useTranslation()
  // 用 dayjs 本地时区格式化，避免 toISOString() 转 UTC 导致日期偏移
  const today = dayjs().format('YYYY-MM-DD')
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Daily Usage Statistics')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <UsageStatsTable
          fetcher={getDailyUsageStats}
          exportBase='/api/usage_statistics'
          initialStartDate={today}
          initialEndDate={today}
          dateLabel={t('Date')}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

// 月用量统计页
export function MonthlyUsageStatistics() {
  const { t } = useTranslation()
  // 用 dayjs 本地时区格式化，避免 toISOString() 转 UTC 导致月份偏移
  const thisMonth = dayjs().format('YYYY-MM')
  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Monthly Usage Statistics')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <UsageStatsTable
          fetcher={getMonthlyUsageStats}
          exportBase='/api/usage_statistics_monthly'
          initialStartDate={thisMonth}
          initialEndDate={thisMonth}
          dateLabel={t('Month')}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
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
