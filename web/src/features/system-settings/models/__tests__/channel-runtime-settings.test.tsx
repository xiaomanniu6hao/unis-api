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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route as ModelsRoute } from '@/routes/_authenticated/system-settings/models/$section'

import { ModelSettings } from '..'

type ModelsBeforeLoad = (context: { params: { section: string } }) => void

// 这些是数据库里已持久化的值，页面必须原样回显，而不是退回内置默认值。
const persistedOptions: Record<string, string> = {
  'channel_runtime_setting.enabled': 'true',
  'channel_runtime_setting.scrape_interval_seconds': '3',
  'channel_runtime_setting.scrape_timeout_seconds': '2',
  'channel_runtime_setting.default_max_batch': '8',
  'channel_runtime_setting.saturation_waiting_samples': '2',
  'channel_runtime_setting.saturation_max_lock_seconds': '600',
}

let queryClient: QueryClient

async function renderModels(path: string) {
  const root = createRootRoute()
  const authenticated = createRoute({
    getParentRoute: () => root,
    id: '_authenticated',
  })
  const sectionRoute = createRoute({
    getParentRoute: () => authenticated,
    path: '/system-settings/models/$section',
    beforeLoad: ModelsRoute.options.beforeLoad as ModelsBeforeLoad,
    component: ModelSettings,
  })
  const router = createRouter({
    routeTree: root.addChildren([
      authenticated.addChildren([sectionRoute]),
    ]),
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await act(() => router.load())
}

function inputForLabel(label: string) {
  const field = screen.getByText(label).parentElement
  if (!field) throw new Error(`no field wrapper for ${label}`)
  return within(field).getByRole('spinbutton')
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/option/') {
      return {
        data: {
          success: true,
          data: Object.entries(persistedOptions).map(([key, value]) => ({
            key,
            value,
          })),
        },
      }
    }
    return { data: { success: true, data: [] } }
  })
})

afterEach(() => {
  queryClient.clear()
})

test('channel runtime section shows the saved options instead of the built-in defaults', async () => {
  await renderModels('/system-settings/models/channel-runtime')

  expect(
    await screen.findByRole('heading', { name: 'Channel Runtime Configuration' })
  ).toBeVisible()

  // 启用采集在库里是 true，默认值是 false。
  expect(screen.getAllByRole('switch')[0]).toBeChecked()
  // 采集间隔库里是 3，默认值是 5。
  expect(inputForLabel('Scrape interval (seconds)')).toHaveValue(3)
  expect(inputForLabel('Scrape timeout (seconds)')).toHaveValue(2)
  // 默认批量库里是 8，默认值是 4。
  expect(inputForLabel('Default max batch')).toHaveValue(8)
  expect(inputForLabel('Waiting samples before lock')).toHaveValue(2)
  expect(inputForLabel('Max lock seconds')).toHaveValue(600)
})

test('channel runtime section only holds configuration, runtime table moved to the dashboard', async () => {
  await renderModels('/system-settings/models/channel-runtime')
  await screen.findByRole('heading', { name: 'Channel Runtime Configuration' })

  expect(screen.queryByRole('heading', { name: 'Current runtime' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Refresh status' })).toBeNull()
})
