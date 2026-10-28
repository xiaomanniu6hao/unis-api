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
import { useQuery } from '@tanstack/react-query'

import { api } from '@/lib/api'

export type TokenItem = { id: number; name: string }

// /api/data/tokens 轻量接口：只返回当前用户自己创建的令牌，
// 不返回其他用户的令牌。
async function fetchTokensForSettings(): Promise<TokenItem[]> {
  const res = await api.get<{
    success: boolean
    data?: TokenItem[]
  }>('/api/data/tokens')
  if (res.data?.success && Array.isArray(res.data.data)) {
    return res.data.data
  }
  return []
}

export function useAllTokens() {
  return useQuery({
    queryKey: ['all-tokens'],
    queryFn: fetchTokensForSettings,
    staleTime: 5 * 60 * 1000,
  })
}

// 把逗号分隔的 id 字符串拆成去重的 id 字符串数组
export function parseTokenIds(raw: string): string[] {
  if (!raw) return []
  const seen = new Set<string>()
  const result: string[] = []
  for (const part of raw.split(',')) {
    const id = part.trim()
    if (!id || seen.has(id)) continue
    seen.add(id)
    result.push(id)
  }
  return result
}