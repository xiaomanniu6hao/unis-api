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

import { formatCompactNumber, formatQuota } from '@/lib/format'
import type { UsageSummary } from '../api'

type StatsSummaryCardsProps = {
  summary: UsageSummary | null | undefined
}

// 统计页顶部 KPI 卡片：请求/Token/额度汇总。
export function StatsSummaryCards({ summary }: StatsSummaryCardsProps) {
  const { t } = useTranslation()

  const cards = [
    {
      label: t('Total Requests'),
      value: summary ? formatCompactNumber(summary.total_requests) : '-',
    },
    {
      label: t('Successful'),
      value: summary ? formatCompactNumber(summary.successful_requests) : '-',
    },
    {
      label: t('Failed'),
      value: summary ? formatCompactNumber(summary.failed_requests) : '-',
    },
    {
      label: t('Total Tokens'),
      value: summary ? formatCompactNumber(summary.total_tokens) : '-',
    },
    {
      label: t('Prompt Tokens'),
      value: summary ? formatCompactNumber(summary.prompt_tokens) : '-',
    },
    {
      label: t('Completion Tokens'),
      value: summary ? formatCompactNumber(summary.completion_tokens) : '-',
    },
    {
      label: t('Cache Tokens'),
      value: summary ? formatCompactNumber(summary.prompt_tokens_cache) : '-',
    },
    {
      label: t('Total Quota'),
      value: summary ? formatQuota(summary.total_quota) : '-',
    },
  ]

  return (
    <div className='grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8'>
      {cards.map((card) => (
        <div
          key={card.label}
          className='bg-card rounded-lg border p-3'
        >
          <div className='text-muted-foreground truncate text-xs'>
            {card.label}
          </div>
          <div className='mt-1 text-lg font-semibold tabular-nums'>
            {card.value}
          </div>
        </div>
      ))}
    </div>
  )
}
