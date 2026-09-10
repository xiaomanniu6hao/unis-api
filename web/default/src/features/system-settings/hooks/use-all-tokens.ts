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

// 后端 page_size 截断到 100，分页循环拉全量，确保排在 100 之后的令牌
// （如 embed/rerank）也能在排除下拉里出现。
async function fetchAllTokens(): Promise<TokenItem[]> {
  const pageSize = 100
  const result: TokenItem[] = []
  let page = 1
  while (page <= 50) {
    const res = await api.get('/api/token/', {
      params: { p: page, size: pageSize },
    })
    const items = (res.data?.data?.items ?? []) as TokenItem[]
    result.push(...items)
    if (items.length < pageSize) break
    page += 1
  }
  return result
}

export function useAllTokens() {
  return useQuery({
    queryKey: ['all-tokens'],
    queryFn: fetchAllTokens,
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
