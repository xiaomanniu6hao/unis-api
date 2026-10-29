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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { ChannelResourceAnalytics } from '@/features/dashboard/components/channels/channel-resource-analytics'

const vchart = vi.hoisted(() =>
  vi.fn((_props: { spec: Record<string, unknown> }) => null)
)
vi.mock('@visactor/react-vchart', () => ({ VChart: vchart }))
vi.mock('@visactor/vchart', () => ({
  ThemeManager: { setCurrentTheme: vi.fn() },
}))

const dashboardApi = vi.hoisted(() => ({
  getChannelRuntimeStatus: vi.fn(),
  getChannelRuntimeSeries: vi.fn(),
  getChannelUsageSeries: vi.fn(),
  refreshChannelRuntime: vi.fn(),
  unlockChannelRuntime: vi.fn(),
}))
vi.mock('@/features/dashboard/api', () => dashboardApi)

const bucket = 1789387200

beforeEach(() => {
  dashboardApi.getChannelRuntimeStatus.mockResolvedValue({
    success: true,
    data: {
      enabled: true,
      saturation_lock_enabled: true,
      length_routing_enabled: false,
      scrape_interval_seconds: 5,
      items: [
        {
          channel_id: 1,
          channel_name: 'gpu-a',
          channel_status: 1,
          models: ['llama-70b'],
          metrics_url: 'http://gpu-a/metrics',
          running: 6,
          waiting: 2,
          max_batch: 8,
          max_batch_source: 'override',
          peak_running: 8,
          kv_cache_usage: 62.5,
          prefix_cache_hit_rate: 31.2,
          locked: false,
          lock_reason: '',
          locked_at: 0,
          candidate_count: 2,
          last_collected_at: bucket,
          last_error: '',
          stale: false,
        },
      ],
    },
  })
  dashboardApi.getChannelUsageSeries.mockResolvedValue({
    success: true,
    data: {
      start_timestamp: bucket,
      end_timestamp: bucket + 3600,
      bucket_seconds: 3600,
      items: [
        {
          channel_id: 1,
          channel_name: 'gpu-a',
          model_names: 'llama-70b',
          bucket,
          requests: 12,
          prompt_tokens: 1000,
          completion_tokens: 500,
        },
      ],
    },
  })
  dashboardApi.getChannelRuntimeSeries.mockResolvedValue({
    success: true,
    data: {
      start_timestamp: bucket,
      end_timestamp: bucket + 3600,
      bucket_seconds: 300,
      channels: [{ id: 1, name: 'gpu-a', models: ['llama-70b'], status: 1 }],
      items: [
        {
          channel_id: 1,
          channel_name: 'gpu-a',
          model_names: 'llama-70b',
          bucket,
          avg_running: 6,
          max_running: 8,
          avg_waiting: 2,
          max_waiting: 3,
          max_batch: 8,
          avg_requests: 8,
          prompt_throughput: 3275.9,
          generation_throughput: 716.7,
          sample_count: 6,
        },
      ],
    },
  })
})

test('shows current runtime, call analytics and per-channel trends together', async () => {
  render(<ChannelResourceAnalytics />)

  expect(
    await screen.findByRole('heading', { name: 'Current runtime' })
  ).toBeVisible()
  // 当前运行时表格里能看到实时 Running / Waiting
  expect(await screen.findByText('#1 gpu-a')).toBeVisible()
  // 调用分析复用模型调用分析的标题与页签
  expect(await screen.findByText('Channel Call Analytics')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Call Trend' })).toBeVisible()
  // 每个渠道一张趋势卡片
  expect(
    await screen.findByRole('button', { name: 'Concurrency' })
  ).toBeVisible()
})

test('reloads usage and runtime trends when the time preset changes', async () => {
  const user = userEvent.setup()
  render(<ChannelResourceAnalytics />)
  await screen.findByText('Channel Call Analytics')
  dashboardApi.getChannelUsageSeries.mockClear()
  dashboardApi.getChannelRuntimeSeries.mockClear()

  await user.click(screen.getByRole('button', { name: 'This week' }))

  await waitFor(() =>
    expect(dashboardApi.getChannelUsageSeries).toHaveBeenCalledWith(
      expect.objectContaining({ bucket_seconds: 86400 })
    )
  )
  const usageParams = dashboardApi.getChannelUsageSeries.mock.calls[0][0]
  const seriesParams = dashboardApi.getChannelRuntimeSeries.mock.calls[0][0]
  // 两个图表共用同一个时间窗
  expect(seriesParams.start_timestamp).toBe(usageParams.start_timestamp)
  expect(seriesParams.end_timestamp).toBe(usageParams.end_timestamp)
  // 当周从周一 00:00 开始
  const weekStart = new Date(usageParams.start_timestamp * 1000)
  expect(weekStart.getDay()).toBe(1)
  expect(weekStart.getHours()).toBe(0)
  expect(seriesParams.bucket_seconds).toBe(1800)
})

test('reloads usage and runtime trends when the runtime refresh button is used', async () => {
  const user = userEvent.setup()
  dashboardApi.refreshChannelRuntime.mockResolvedValue({
    success: true,
    message: '',
  })
  render(<ChannelResourceAnalytics />)
  await screen.findByText('Channel Call Analytics')
  await waitFor(() =>
    expect(dashboardApi.getChannelUsageSeries).toHaveBeenCalled()
  )
  dashboardApi.getChannelUsageSeries.mockClear()
  dashboardApi.getChannelRuntimeSeries.mockClear()

  await user.click(screen.getByRole('button', { name: 'Refresh status' }))

  // 刷新采集后，调用分析与趋势两块数据都要重新拉取
  await waitFor(() =>
    expect(dashboardApi.getChannelUsageSeries).toHaveBeenCalledTimes(1)
  )
  expect(dashboardApi.getChannelRuntimeSeries).toHaveBeenCalledTimes(1)
})

test('opens the custom range dialog with start and end pickers', async () => {
  const user = userEvent.setup()
  render(<ChannelResourceAnalytics />)
  await screen.findByText('Channel Call Analytics')

  await user.click(screen.getByRole('button', { name: 'Custom' }))

  expect(await screen.findByText('Custom time range')).toBeVisible()
  expect(screen.getByText('Start time')).toBeVisible()
  expect(screen.getByText('End time')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Apply' })).toBeVisible()
})
