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

import { ChannelRuntimeTrends } from '@/features/dashboard/components/channels/channel-runtime-trends'
import { resolveChannelTimeRange } from '@/features/dashboard/lib/channel-range'

const vchart = vi.hoisted(() =>
  vi.fn((_props: { spec: Record<string, unknown> }) => null)
)
vi.mock('@visactor/react-vchart', () => ({ VChart: vchart }))
vi.mock('@visactor/vchart', () => ({
  ThemeManager: { setCurrentTheme: vi.fn() },
}))

const dashboardApi = vi.hoisted(() => ({
  getChannelRuntimeSeries: vi.fn(),
}))
vi.mock('@/features/dashboard/api', () => dashboardApi)

const bucket = 1789387200
const seriesResponse = {
  success: true,
  data: {
    start_timestamp: bucket,
    end_timestamp: bucket + 3600,
    bucket_seconds: 300,
    channels: [
      { id: 2, name: 'gpu-b', models: ['qwen3-32b'], status: 1 },
      { id: 1, name: 'gpu-a', models: ['llama-70b'], status: 1 },
    ],
    items: [
      {
        channel_id: 1,
        channel_name: 'gpu-a',
        model_names: 'llama-70b',
        bucket,
        avg_running: 6,
        max_running: 8,
        avg_waiting: 1,
        max_waiting: 2,
        max_batch: 8,
        avg_requests: 7,
        prompt_throughput: 3275.9,
        generation_throughput: 716.7,
        sample_count: 6,
      },
    ],
  },
}

beforeEach(() => {
  dashboardApi.getChannelRuntimeSeries.mockResolvedValue(seriesResponse)
})

test('renders one card per channel and no-data state for channels without samples', async () => {
  render(<ChannelRuntimeTrends range={resolveChannelTimeRange('day')} />)

  expect(await screen.findByText('gpu-a')).toBeVisible()
  expect(screen.getByText('gpu-b')).toBeVisible()
  // 没有样本的渠道也要给出明确空状态，而不是空白卡片
  expect(screen.getByText('No data available')).toBeVisible()
  expect(screen.getAllByRole('button', { name: 'Concurrency' })).toHaveLength(2)
})

test('switches every channel card to the prefill throughput series', async () => {
  const user = userEvent.setup()
  render(<ChannelRuntimeTrends range={resolveChannelTimeRange('day')} />)
  await screen.findByText('gpu-a')

  await user.click(
    screen.getAllByRole('button', {
      name: 'Prefill performance',
    })[0]
  )

  await waitFor(() => expect(vchart).toHaveBeenCalled())
  const spec = vchart.mock.calls.at(-1)?.[0].spec as {
    title?: { text?: string }
    data: { values: { Series: string; Value: number }[] }[]
  }
  expect(spec.data[0].values[0].Series).toBe('Prefill performance')
  expect(spec.data[0].values[0].Value).toBe(3275.9)
  // 图表标题与 tooltip 统一使用 tokens/s
  expect(spec.title?.text).toBe('Prefill performance (tokens/s)')
})

test('requests a range-aligned bucket size for the selected window', async () => {
  const range = resolveChannelTimeRange('month')
  render(<ChannelRuntimeTrends range={range} />)

  await waitFor(() =>
    expect(dashboardApi.getChannelRuntimeSeries).toHaveBeenCalledWith({
      start_timestamp: Math.floor(range.start.getTime() / 1000),
      end_timestamp: Math.floor(range.end.getTime() / 1000),
      bucket_seconds: 7200,
    })
  )
})
