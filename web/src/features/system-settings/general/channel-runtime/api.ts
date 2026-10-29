import { api } from '@/lib/api'

import type { ChannelRuntimeChannelItem } from './types'

// 环境中的全部渠道（不含密钥），用于把配置默认行预填成真实渠道
export async function getChannelRuntimeChannels(): Promise<{
  success: boolean
  message?: string
  data?: ChannelRuntimeChannelItem[]
}> {
  const res = await api.get('/api/channel_runtime/channels', {
    disableDuplicate: true,
  } as Record<string, unknown>)
  return res.data
}
