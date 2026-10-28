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

// /api/channel/models_enabled 返回全系统已启用的模型名列表，
// 用于并发限制按模型覆盖时选择模型。
async function fetchEnabledModels(): Promise<string[]> {
  const res = await api.get<{ success: boolean; data?: string[] }>(
    '/api/channel/models_enabled'
  )
  if (res.data?.success && Array.isArray(res.data.data)) {
    return res.data.data
  }
  return []
}

export function useEnabledModels() {
  return useQuery({
    queryKey: ['enabled-models'],
    queryFn: fetchEnabledModels,
    staleTime: 5 * 60 * 1000,
  })
}
