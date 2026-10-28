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
import { api } from '@/lib/api'

import type { ChannelResourceResponse } from './components/channels/types'
import type {
  FlowQuotaDataItem,
  QuotaDataItem,
  TokenQuotaDataItem,
  UptimeGroupResult,
} from './types'

// ============================================================================
// Dashboard APIs
// ============================================================================

// ----------------------------------------------------------------------------
// Quota & Usage Data
// ----------------------------------------------------------------------------

// Get user quota data within a time range
// Admin users get all users' data by default.
export async function getUserQuotaDates(
  params: {
    start_timestamp: number
    end_timestamp: number
    default_time?: string
    username?: string
    excluded_token_ids?: string
  },
  isAdmin = false
) {
  const endpoint = isAdmin ? '/api/data' : '/api/data/self'
  const res = await api.get<{ success: boolean; data: QuotaDataItem[] }>(
    endpoint,
    { params }
  )
  return res.data
}

// 数据看板筛选用的轻量令牌列表（id + name）
export async function getDataFilterTokens() {
  const res = await api.get<{
    success: boolean
    data: { id: number; name: string }[]
  }>('/api/data/tokens')
  return res.data
}

// ----------------------------------------------------------------------------
// System Monitoring
// ----------------------------------------------------------------------------

export async function getUserQuotaDataByUsers(params: {
  start_timestamp: number
  end_timestamp: number
  excluded_token_ids?: string
}) {
  const res = await api.get<{ success: boolean; data: QuotaDataItem[] }>(
    '/api/data/users',
    { params }
  )
  return res.data
}

// 令牌统计：按令牌聚合的当前用户自己的额度数据
export async function getTokenQuotaDataByTokens(params: {
  start_timestamp: number
  end_timestamp: number
  excluded_token_ids?: string
}) {
  const res = await api.get<{ success: boolean; data: TokenQuotaDataItem[] }>(
    '/api/data/tokens/stat',
    { params }
  )
  return res.data
}

export async function getFlowQuotaDates(
  params: {
    start_timestamp: number
    end_timestamp: number
    default_time?: string
    username?: string
    excluded_token_ids?: string
  },
  isAdmin = false
) {
  const endpoint = isAdmin ? '/api/data/flow' : '/api/data/flow/self'
  const res = await api.get<{
    success: boolean
    data?: FlowQuotaDataItem[]
    message?: string
  }>(endpoint, { params })
  return res.data
}

// Get uptime monitoring status for all services
export async function getUptimeStatus() {
  const res = await api.get<{ success: boolean; data: UptimeGroupResult[] }>(
    '/api/uptime/status'
  )
  return res.data
}

// ----------------------------------------------------------------------------
// Channel Resource Analytics
// ----------------------------------------------------------------------------

// 按渠道聚合的调用情况 + 当前 Running/Waiting/最大 batch（渠道资源分析页）
export async function getChannelResourceStats(hours = 24) {
  const res = await api.get<{ success: boolean; data: ChannelResourceResponse }>(
    '/api/channel_runtime/resource',
    { params: { hours } }
  )
  return res.data
}
