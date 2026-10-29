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

import { CurrentRuntimePanel } from '@/features/dashboard/components/channels/current-runtime-panel'

const dashboardApi = vi.hoisted(() => ({
  getChannelRuntimeStatus: vi.fn(),
  refreshChannelRuntime: vi.fn(),
  unlockChannelRuntime: vi.fn(),
}))
vi.mock('@/features/dashboard/api', () => dashboardApi)

const statusItem = {
  channel_id: 3,
  channel_name: 'gpu-c',
  channel_status: 1,
  models: ['qwen3-32b'],
  metrics_url: 'http://gpu-c/metrics',
  running: 8,
  waiting: 4,
  max_batch: 8,
  max_batch_source: 'override',
  peak_running: 8,
  kv_cache_usage: 91.5,
  prefix_cache_hit_rate: 12.4,
  locked: true,
  lock_reason: 'waiting',
  locked_at: 1789387200,
  candidate_count: 2,
  last_collected_at: 1789387200,
  last_error: '',
  stale: false,
}

beforeEach(() => {
  dashboardApi.getChannelRuntimeStatus.mockResolvedValue({
    success: true,
    data: {
      enabled: true,
      saturation_lock_enabled: true,
      length_routing_enabled: false,
      scrape_interval_seconds: 5,
      items: [statusItem],
    },
  })
  dashboardApi.refreshChannelRuntime.mockResolvedValue({ success: true })
  dashboardApi.unlockChannelRuntime.mockResolvedValue({ success: true })
})

test('unlocking a locked channel calls the unlock endpoint with its id', async () => {
  const user = userEvent.setup()
  render(<CurrentRuntimePanel />)

  expect(await screen.findByText('#3 gpu-c')).toBeVisible()
  expect(screen.getByText('Locked')).toBeVisible()

  await user.click(screen.getByRole('button', { name: 'Unlock' }))

  await waitFor(() =>
    expect(dashboardApi.unlockChannelRuntime).toHaveBeenCalledWith(3)
  )
})

test('refresh button triggers a new collection', async () => {
  const user = userEvent.setup()
  render(<CurrentRuntimePanel />)
  await screen.findByText('#3 gpu-c')

  await user.click(screen.getByRole('button', { name: 'Refresh status' }))

  await waitFor(() =>
    expect(dashboardApi.refreshChannelRuntime).toHaveBeenCalled()
  )
})

test('shows an explicit empty state when no runtime data was collected', async () => {
  dashboardApi.getChannelRuntimeStatus.mockResolvedValue({
    success: true,
    data: {
      enabled: false,
      saturation_lock_enabled: true,
      length_routing_enabled: false,
      scrape_interval_seconds: 5,
      items: [],
    },
  })

  render(<CurrentRuntimePanel />)

  expect(
    await screen.findByText(
      'No runtime data yet. Enable collection and wait one interval.'
    )
  ).toBeVisible()
})
