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

import type {
  ChannelRuntimeSeriesResponse,
  ChannelRuntimeStatusResponse,
  ChannelSeriesResponse,
  ChannelUsageSeriesItem,
} from './components/channels/types'
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

// 各渠道当前运行时状态（Running/Waiting/锁状态），看板「当前运行时」表格
export async function getChannelRuntimeStatus() {
  const res = await api.get<{
    success: boolean
    message?: string
    data?: ChannelRuntimeStatusResponse
  }>('/api/channel_runtime/status', {
    disableDuplicate: true,
  } as Record<string, unknown>)
  return res.data
}

// 按时间桶聚合的运行时趋势（Running/Waiting/MaxBatch/prefill/decode 吞吐）
export async function getChannelRuntimeSeries(params: {
  start_timestamp: number
  end_timestamp: number
  bucket_seconds: number
}) {
  const res = await api.get<{
    success: boolean
    message?: string
    data?: ChannelRuntimeSeriesResponse
  }>('/api/channel_runtime/series', { params })
  return res.data
}

// 按渠道 + 时间桶聚合的调用量（渠道调用分析的趋势/分布/排行）
export async function getChannelUsageSeries(params: {
  start_timestamp: number
  end_timestamp: number
  bucket_seconds: number
}) {
  const res = await api.get<{
    success: boolean
    message?: string
    data?: ChannelSeriesResponse<ChannelUsageSeriesItem>
  }>('/api/channel_runtime/usage_series', { params })
  return res.data
}

// 立即触发一次采集
export async function refreshChannelRuntime() {
  const res = await api.post<{ success: boolean; message?: string }>(
    '/api/channel_runtime/refresh'
  )
  return res.data
}

// 手工解除渠道饱和锁
export async function unlockChannelRuntime(channelId: number) {
  const res = await api.post<{ success: boolean; message?: string }>(
    `/api/channel_runtime/${channelId}/unlock`
  )
  return res.data
}
