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

export type InfoToken = {
  id: number
  name: string
  key: string
  status: number
  remain_quota: number
  used_quota: number
  unlimited_quota: boolean
  group: string
  expired_time: number
  created_time: number
}

type InfoTokensResponse = {
  success: boolean
  message?: string
  data: {
    items: InfoToken[]
    total: number
    page: number
    page_size: number
  }
}

type UpdateOptionResponse = {
  success: boolean
  message?: string
}

// /info 页拉取明文 token 列表。后端 /api/token-info/ 在 TryUserAuth 下：
// 未认证返回全部 token，已认证返回该用户 token（controller 不分页，直接返回全量）。
// 这是 MIXAPI /info 的固有行为。
export async function getInfoTokens(): Promise<InfoTokensResponse> {
  const res = await api.get('/api/token-info/')
  return res.data
}

// root 用户编辑公告：PUT /api/option/ { key: 'Notice', value }
export async function updateNotice(value: string): Promise<UpdateOptionResponse> {
  const res = await api.put('/api/option/', { key: 'Notice', value })
  return res.data
}
