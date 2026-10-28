import { api } from '@/lib/api'

import type {
  ChannelRuntimeLimitItem,
  ChannelRuntimeStatusResponse,
} from './types'

export async function getChannelRuntimeStatus(): Promise<{
  success: boolean
  message?: string
  data?: ChannelRuntimeStatusResponse
}> {
  const res = await api.get('/api/channel_runtime/status', {
    disableDuplicate: true,
  } as Record<string, unknown>)
  return res.data
}

export async function getChannelRuntimeLimits(): Promise<{
  success: boolean
  message?: string
  data?: ChannelRuntimeLimitItem[]
}> {
  const res = await api.get('/api/channel_runtime/limits', {
    disableDuplicate: true,
  } as Record<string, unknown>)
  return res.data
}

export async function refreshChannelRuntime(): Promise<{
  success: boolean
  message?: string
}> {
  const res = await api.post('/api/channel_runtime/refresh')
  return res.data
}

export async function unlockChannelRuntime(
  channelId: number
): Promise<{ success: boolean; message?: string }> {
  const res = await api.post(`/api/channel_runtime/${channelId}/unlock`)
  return res.data
}
