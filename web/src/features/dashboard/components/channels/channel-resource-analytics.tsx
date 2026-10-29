import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ChannelCallAnalytics } from '@/features/dashboard/components/channels/channel-call-analytics'
import { ChannelRuntimeTrends } from '@/features/dashboard/components/channels/channel-runtime-trends'
import { ChannelTimeRangePicker } from '@/features/dashboard/components/channels/channel-time-range'
import { CurrentRuntimePanel } from '@/features/dashboard/components/channels/current-runtime-panel'
import {
  type ChannelTimeRange,
  resolveChannelTimeRange,
} from '@/features/dashboard/lib/channel-range'

export function ChannelResourceAnalytics() {
  const { t } = useTranslation()
  // 时间范围由调用分析与运行时趋势共用，切换后两块一起刷新。
  const [range, setRange] = useState<ChannelTimeRange>(() =>
    resolveChannelTimeRange('day')
  )
  // 刷新采集后递增，调用分析与渠道趋势据此重新拉取数据。
  const [refreshKey, setRefreshKey] = useState(0)

  return (
    <div className='space-y-3 sm:space-y-4'>
      <div className='flex flex-wrap items-center justify-end gap-2'>
        <span className='text-muted-foreground text-xs'>{t('Time range')}</span>
        <ChannelTimeRangePicker value={range} onChange={setRange} />
      </div>

      <CurrentRuntimePanel onRefresh={() => setRefreshKey((key) => key + 1)} />
      <ChannelCallAnalytics range={range} refreshKey={refreshKey} />
      <ChannelRuntimeTrends range={range} refreshKey={refreshKey} />
    </div>
  )
}
