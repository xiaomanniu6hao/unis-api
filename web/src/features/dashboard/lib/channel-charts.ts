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

// 调用并发数：最大 batch（横线）+ Running + 请求数（Running + Waiting）。
export function buildChannelConcurrencySpec(
  points: ChannelRuntimeSeriesItem[],
  bucketSeconds: number,
  t: TFunction
): ChartSpec {
  const seriesNames = [t('Max batch'), 'Running', t('Requests')]
  const values: TrendPoint[] = []
  points.forEach((point) => {
    const time = formatChannelBucket(bucketSeconds, point.bucket)
    values.push(
      {
        Time: time,
        Series: t('Max batch'),
        Value: point.max_batch,
      },
      {
        Time: time,
        Series: 'Running',
        Value: Number(point.avg_running.toFixed(2)),
      },
      {
        Time: time,
        Series: t('Requests'),
        Value: Number(point.avg_requests.toFixed(2)),
      }
    )
  })
  return buildLineSpec(
    values,
    seriesNames,
    t('Concurrency'),
    'channel-concurrency',
    t
  )
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
