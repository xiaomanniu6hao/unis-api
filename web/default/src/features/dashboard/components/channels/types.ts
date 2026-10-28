export interface ChannelResourceItem {
  channel_id: number
  channel_name: string
  model_names: string
  status: number

  requests: number
  error_requests: number
  error_rate: number
  quota: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  avg_use_time_ms: number

  running: number
  waiting: number
  max_batch: number
  max_batch_source: string
  utilization: number
  kv_cache_usage: number
  prefix_cache_hit_rate: number
  locked: boolean
  has_runtime_data: boolean
  last_collected_at: number
}

export interface ChannelResourceResponse {
  hours: number
  enabled: boolean
  saturation_lock_enabled: boolean
  scrape_interval_seconds: number
  items: ChannelResourceItem[]
}
