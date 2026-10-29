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
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { getChannelUsageSeries } from '@/features/dashboard/api'
import type { ChannelUsageSeriesItem } from '@/features/dashboard/components/channels/types'
import { ModelCharts } from '@/features/dashboard/components/models/model-charts'
import {
  channelRangeParams,
  channelUsageBucketSeconds,
  channelUsageGranularity,
  type ChannelTimeRange,
} from '@/features/dashboard/lib/channel-range'
import type { QuotaDataItem } from '@/features/dashboard/types'

interface Props {
  range: ChannelTimeRange
}

// 把「渠道 × 时间桶」的调用量映射成模型调用分析同款数据结构，
// 复用同一套调用趋势 / 分布 / 排行图表。
function toQuotaDataItems(items: ChannelUsageSeriesItem[]): QuotaDataItem[] {
  return items.map((item) => ({
    model_name: item.channel_name || `#${item.channel_id}`,
    created_at: item.bucket,
    count: item.requests,
    token_used: item.prompt_tokens + item.completion_tokens,
    quota: 0,
  }))
}

export function ChannelCallAnalytics(props: Props) {
  const { t } = useTranslation()
  const [items, setItems] = useState<ChannelUsageSeriesItem[]>([])
  const [loading, setLoading] = useState(true)

  const params = useMemo(
    () => ({
      ...channelRangeParams(props.range),
      bucket_seconds: channelUsageBucketSeconds(props.range),
    }),
    [props.range]
  )

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    void getChannelUsageSeries(params)
      .then((res) => {
        if (cancelled) return
        if (!res?.success) {
          toast.error(res?.message || t('Failed to load channel call data'))
          setItems([])
          return
        }
        setItems(res.data?.items || [])
      })
      .catch(() => {
        if (cancelled) return
        toast.error(t('Failed to load channel call data'))
        setItems([])
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [params, t])

  const chartData = useMemo(() => toQuotaDataItems(items), [items])

  return (
    <ModelCharts
      data={chartData}
      loading={loading}
      titleKey='Channel Call Analytics'
      timeGranularity={channelUsageGranularity(props.range)}
    />
  )
}
