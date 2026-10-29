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
import type { TFunction } from 'i18next'
import { describe, expect, test } from 'vitest'

import type { ChannelRuntimeSeriesItem } from '@/features/dashboard/components/channels/types'

import {
  buildChannelConcurrencySpec,
  buildChannelThroughputSpec,
  resolveConcurrencyTier,
} from '../channel-charts'

const t = ((key: string) => key) as unknown as TFunction

function point(
  overrides: Partial<ChannelRuntimeSeriesItem>
): ChannelRuntimeSeriesItem {
  return {
    channel_id: 1,
    channel_name: 'gpu-a',
    model_names: 'llama-70b',
    bucket: 1789387200,
    avg_running: 0,
    max_running: 0,
    avg_waiting: 0,
    max_waiting: 0,
    max_batch: 8,
    avg_requests: 0,
    max_requests: 0,
    prompt_throughput: 0,
    generation_throughput: 0,
    sample_count: 1,
    ...overrides,
  }
}

type TierPoint = { Time: string; Series: string; Value: number }

describe('resolveConcurrencyTier', () => {
  test('maps utilization to the legacy tier thresholds', () => {
    expect(resolveConcurrencyTier(0, 8)).toBe('idle')
    expect(resolveConcurrencyTier(3, 8)).toBe('idle')
    expect(resolveConcurrencyTier(4, 8)).toBe('normal')
    expect(resolveConcurrencyTier(7, 8)).toBe('busy')
    expect(resolveConcurrencyTier(8, 8)).toBe('saturated')
    // 超过上限（等待队列堆积导致 Running 溢出）同样算饱和
    expect(resolveConcurrencyTier(10, 8)).toBe('saturated')
  })

  test('treats unknown max batch as idle instead of dividing by zero', () => {
    expect(resolveConcurrencyTier(6, 0)).toBe('idle')
  })
})

describe('buildChannelConcurrencySpec', () => {
  test('fills the area between Running and zero with the tier color', () => {
    const spec = buildChannelConcurrencySpec(
      [
        point({
          bucket: 1789387200,
          avg_running: 7,
          max_batch: 8,
          max_requests: 9,
        }),
        point({
          bucket: 1789387500,
          avg_running: 8,
          max_batch: 8,
          max_requests: 10,
        }),
      ],
      300,
      t
    )

    expect(spec.type).toBe('common')
    const [areaSeries, lineSeries] = spec.series as Array<{
      type: string
      stack?: boolean
      dataId: string
    }>
    expect(areaSeries.type).toBe('area')
    expect(areaSeries.stack).toBe(true)
    expect(lineSeries.type).toBe('line')

    const data = spec.data as Array<{ id: string; values: TierPoint[] }>
    const tierValues = data[0].values
    // 每个时间点四档都有值，只有当前档位非 0
    const firstBucket = tierValues.slice(0, 4)
    expect(firstBucket.map((item) => item.Series)).toEqual([
      'Idle',
      'Normal',
      'Busy',
      'Saturated',
    ])
    expect(firstBucket.map((item) => item.Value)).toEqual([0, 0, 7, 0])
    const secondBucket = tierValues.slice(4, 8)
    expect(secondBucket.map((item) => item.Value)).toEqual([0, 0, 0, 8])

    // 四条线系列：最大 batch + 在途请求峰值
    expect(data[1].values.map((item) => [item.Series, item.Value])).toEqual([
      ['Max batch', 8],
      ['Peak in-flight', 9],
      ['Max batch', 8],
      ['Peak in-flight', 10],
    ])

    // 颜色顺序必须与图例档位一致，饱和档为红色
    const color = spec.color as { domain: string[]; range: string[] }
    expect(color.domain).toEqual([
      'Idle',
      'Normal',
      'Busy',
      'Saturated',
      'Max batch',
      'Peak in-flight',
    ])
    expect(color.range.slice(0, 4)).toEqual([
      '#10b981',
      '#0ea5e9',
      '#f59e0b',
      '#ef4444',
    ])
  })

  test('describes the empty case in the chart title', () => {
    const spec = buildChannelConcurrencySpec([], 300, t)
    const title = spec.title as { text: string; subtext?: string }
    expect(title.text).toBe('Concurrency')
    expect(title.subtext).toBe('No data available')
  })
})

describe('buildChannelThroughputSpec', () => {
  test('labels prefill / decode throughput with tokens/s', () => {
    const spec = buildChannelThroughputSpec(
      [point({ prompt_throughput: 3275.94, generation_throughput: 716.72 })],
      300,
      'prompt',
      t
    )
    const title = spec.title as { text: string }
    expect(title.text).toBe('Prefill performance (tokens/s)')
    const data = spec.data as Array<{ values: TierPoint[] }>
    expect(data[0].values[0].Value).toBe(3275.9)
  })
})
