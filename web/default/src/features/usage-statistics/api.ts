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
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

// 统计页 API 封装。移植自 MIXAPI 的 6 个统计接口，改用 new-api-dev 的 axios 实例。
// admin 调根路径，普通用户调 /self 路径——与后端 router/api-router.go 的路由组一致。

export type UsageStatisticsItem = {
  id: number
  date: string
  token_id: number
  token_name: string
  model_name: string
  total_requests: number
  successful_requests: number
  failed_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  prompt_tokens_cache: number
  total_quota: number
  avg_prompt_tokens: number
  min_prompt_tokens: number
  max_prompt_tokens: number
  avg_completion_tokens: number
  min_completion_tokens: number
  max_completion_tokens: number
  question_count: number
  avg_requests_per_question: number
  min_requests_per_question: number
  max_requests_per_question: number
}

export type UsageSummary = {
  total_requests: number
  successful_requests: number
  failed_requests: number
  total_tokens: number
  prompt_tokens: number
  completion_tokens: number
  prompt_tokens_cache: number
  total_quota: number
  avg_prompt_tokens: number
  avg_completion_tokens: number
}

export type DistributionItem = {
  token_name: string
  range_grp: string
  count: number
  percent: number
  total_count: number
  total_percent: number
}

export type StatsQuery = {
  start_date?: string
  end_date?: string
  token_id?: number
  token_ids?: string
  model_name?: string
  group_by?: string
}

export type StatsResponse<T> = {
  success: boolean
  message?: string
  data: {
    items: T[]
    total: number
    page: number
    page_size: number
    summary?: UsageSummary
    range_groups?: string[]
  }
}

// 返回当前用户是否管理员（role >= ADMIN）。决定调用 admin 还是 self 端点。
export function useIsAdmin(): boolean {
  const role = useAuthStore((s) => s.auth.user?.role)
  return role != null && role >= ROLE.ADMIN
}

// 拼接 admin/self 端点：admin 用 base，非 admin 用 `${base}/self`。
function endpoint(base: string, isAdmin: boolean, selfPath = '/self'): string {
  return isAdmin ? base : `${base}${selfPath}`
}

// 日用量统计
export async function getDailyUsageStats(
  params: StatsQuery & { p: number; size: number },
  isAdmin: boolean
): Promise<StatsResponse<UsageStatisticsItem>> {
  const res = await api.get(endpoint('/api/usage_statistics/', isAdmin), {
    params,
  })
  return res.data
}

// 月用量统计
export async function getMonthlyUsageStats(
  params: StatsQuery & { p: number; size: number },
  isAdmin: boolean
): Promise<StatsResponse<UsageStatisticsItem>> {
  const res = await api.get(endpoint('/api/usage_statistics_monthly/', isAdmin), {
    params,
  })
  return res.data
}

// 用量排序统计
export async function getRankUsageStats(
  params: StatsQuery & { p: number; size: number },
  isAdmin: boolean
): Promise<StatsResponse<UsageStatisticsItem>> {
  const res = await api.get(endpoint('/api/usage_statistics_rank/', isAdmin), {
    params,
  })
  return res.data
}

// token 分布统计。base 是完整路径前缀：
//   prompt    -> /api/prompt_tokens_distribution/
//   completion -> /api/completion_tokens_distribution/
//   request_count -> /api/request_count_distribution/
export async function getDistributionStats(
  base: string,
  params: StatsQuery & { p: number; size: number },
  isAdmin: boolean
): Promise<StatsResponse<DistributionItem>> {
  const res = await api.get(endpoint(base, isAdmin, '/self'), { params })
  return res.data
}

// CSV 导出：下载 blob 并触发浏览器保存。admin 用 /export，非 admin 用 /self/export。
export async function exportStatsCsv(
  base: string,
  params: StatsQuery,
  isAdmin: boolean,
  filename: string
): Promise<void> {
  const url = endpoint(`${base}/export`, isAdmin, '/self/export')
  const res = await api.get(url, { params, responseType: 'blob' })
  const blob = new Blob([res.data], { type: 'text/csv;charset=utf-8' })
  const downloadUrl = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = downloadUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(downloadUrl)
}

// 拉取 token 列表用于筛选下拉。/api/token/ 自动按当前用户过滤
// （管理员看全部，普通用户看自己的），key 为掩码——下拉只取 id/name。
export type StatsTokenOption = { id: number; name: string }

export async function getStatsTokens(): Promise<StatsTokenOption[]> {
  const res = await api.get('/api/token/', { params: { p: 1, size: 100 } })
  const items = res.data?.data?.items ?? []
  return (items as Array<{ id: number; name: string }>).map((t) => ({
    id: t.id,
    name: t.name,
  }))
}
