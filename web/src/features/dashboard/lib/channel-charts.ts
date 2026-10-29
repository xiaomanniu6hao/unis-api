import type { TFunction } from 'i18next'

import type { ChannelRuntimeSeriesItem } from '@/features/dashboard/components/channels/types'
import dayjs from '@/lib/dayjs'

import { getDashboardChartColors } from './charts'

type ChartSpec = Record<string, unknown>

interface TrendPoint {
  Time: string
  Series: string
  Value: number
}

// 桶宽决定时间轴的展示精度：分钟级桶只显示时分，跨天时补上日期。
export function formatChannelBucket(
  bucketSeconds: number,
  bucket: number
): string {
  const time = dayjs(bucket * 1000)
  if (bucketSeconds < 3600) {
    return bucketSeconds < 600
      ? time.format('HH:mm')
      : time.format('MM-DD HH:mm')
  }
  if (bucketSeconds < 86400) return time.format('MM-DD HH:mm')
  return time.format('MM-DD')
}

function buildLineSpec(
  values: TrendPoint[],
  seriesNames: string[],
  title: string,
  dataId: string,
  t: TFunction,
  valueUnit?: string
): ChartSpec {
  return {
    type: 'line',
    data: [{ id: dataId, values }],
    xField: 'Time',
    yField: 'Value',
    seriesField: 'Series',
    legends: { visible: true, selectMode: 'multiple' },
    color: {
      type: 'ordinal',
      domain: seriesNames,
      range: getDashboardChartColors(seriesNames.length),
    },
    line: { style: { lineWidth: 2, curveType: 'monotone' } },
    point: { visible: false },
    title: {
      visible: true,
      text: title,
      subtext: values.length === 0 ? t('No data available') : undefined,
    },
    tooltip: {
      dimension: {
        content: [
          {
            key: (datum: Record<string, unknown>) => datum?.Series,
            value: (datum: Record<string, unknown>) =>
              valueUnit ? `${datum?.Value} ${valueUnit}` : datum?.Value,
          },
        ],
      },
    },
    background: { fill: 'transparent' },
    animation: true,
  }
}

// 并发拥挤度分档：与旧版渠道资源看板的色标保持一致
// （空闲绿 / 正常蓝 / 繁忙黄 / 饱和红）。
export type ConcurrencyTier = 'idle' | 'normal' | 'busy' | 'saturated'

export const CONCURRENCY_TIERS: Array<{
  tier: ConcurrencyTier
  labelKey: string
  color: string
}> = [
  { tier: 'idle', labelKey: 'Idle', color: '#10b981' },
  { tier: 'normal', labelKey: 'Normal', color: '#0ea5e9' },
  { tier: 'busy', labelKey: 'Busy', color: '#f59e0b' },
  { tier: 'saturated', labelKey: 'Saturated', color: '#ef4444' },
]

// 按 Running / 最大 batch 的利用率分档，max_batch 未知时按空闲处理。
export function resolveConcurrencyTier(
  running: number,
  maxBatch: number
): ConcurrencyTier {
  if (maxBatch <= 0) return 'idle'
  const utilization = (running / maxBatch) * 100
  if (utilization >= 100) return 'saturated'
  if (utilization >= 80) return 'busy'
  if (utilization >= 50) return 'normal'
  return 'idle'
}

// 调用并发数：Running 曲线到 0 轴之间的区域按拥挤度着色，
// 叠加最大 batch 与在途请求数（Running + Waiting 的桶内峰值）两条参考线。
export function buildChannelConcurrencySpec(
  points: ChannelRuntimeSeriesItem[],
  bucketSeconds: number,
  t: TFunction
): ChartSpec {
  const tierNames = CONCURRENCY_TIERS.map((item) => t(item.labelKey))
  const lineColors = ['#64748b', '#7c3aed']
  // 在途请求数取桶内峰值（整数观测值），不用均值：均值是把多个瞬时采样
  // 平均出来的，会出现 7.83 这种小数，看着不像“请求数”。
  const lineNames = [t('Max batch'), t('Peak in-flight')]
  const tierValues: TrendPoint[] = []
  const lineValues: TrendPoint[] = []

  points.forEach((point) => {
    const time = formatChannelBucket(bucketSeconds, point.bucket)
    const running = Number(point.avg_running.toFixed(2))
    const activeTier = resolveConcurrencyTier(
      point.avg_running,
      point.max_batch
    )
    // 同一时刻只有当前档位有值，堆叠后 0→Running 整块就是该档位的颜色。
    CONCURRENCY_TIERS.forEach((item) => {
      tierValues.push({
        Time: time,
        Series: t(item.labelKey),
        Value: item.tier === activeTier ? running : 0,
      })
    })
    lineValues.push({
      Time: time,
      Series: t('Max batch'),
      Value: point.max_batch,
    })
    lineValues.push({
      Time: time,
      Series: t('Peak in-flight'),
      Value: Math.round(point.max_requests),
    })
  })

  return {
    type: 'common',
    data: [
      { id: 'channel-concurrency-tier', values: tierValues },
      { id: 'channel-concurrency-line', values: lineValues },
    ],
    series: [
      {
        type: 'area',
        dataId: 'channel-concurrency-tier',
        xField: 'Time',
        yField: 'Value',
        seriesField: 'Series',
        stack: true,
        // 实心填充：图例色块与实际区域颜色必须一致，不能靠透明度减淡。
        area: { style: { fillOpacity: 1 } },
        line: { style: { lineWidth: 0 } },
        point: { visible: false },
      },
      {
        type: 'line',
        dataId: 'channel-concurrency-line',
        xField: 'Time',
        yField: 'Value',
        seriesField: 'Series',
        line: { style: { lineWidth: 2, curveType: 'monotone' } },
        point: { visible: false },
      },
    ],
    axes: [
      { orient: 'left', type: 'linear' },
      { orient: 'bottom', type: 'band' },
    ],
    legends: { visible: true, selectMode: 'multiple' },
    color: {
      type: 'ordinal',
      domain: [...tierNames, ...lineNames],
      range: [...CONCURRENCY_TIERS.map((item) => item.color), ...lineColors],
    },
    title: {
      visible: true,
      text: t('Concurrency'),
      subtext: points.length === 0 ? t('No data available') : undefined,
    },
    background: { fill: 'transparent' },
    animation: true,
  }
}

// Prefill / decode 性能：各一条吞吐曲线，单位统一为 tokens/s。
export function buildChannelThroughputSpec(
  points: ChannelRuntimeSeriesItem[],
  bucketSeconds: number,
  metric: 'prompt' | 'generation',
  t: TFunction
): ChartSpec {
  const title =
    metric === 'prompt' ? t('Prefill performance') : t('Decode performance')
  const unit = 'tokens/s'
  const values = points.map((point) => ({
    Time: formatChannelBucket(bucketSeconds, point.bucket),
    Series: title,
    Value: Number(
      (metric === 'prompt'
        ? point.prompt_throughput
        : point.generation_throughput
      ).toFixed(1)
    ),
  }))
  return buildLineSpec(
    values,
    [title],
    `${title} (${unit})`,
    `channel-${metric}`,
    t,
    unit
  )
}
