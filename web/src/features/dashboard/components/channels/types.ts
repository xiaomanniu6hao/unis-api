export interface ChannelRuntimeStatusItem {
  channel_id: number
  channel_name: string
  channel_status: number
  models: string[]
  metrics_url: string
  running: number
  waiting: number
  max_batch: number
  max_batch_source: string
  peak_running: number
  kv_cache_usage: number
  prefix_cache_hit_rate: number
  locked: boolean
  lock_reason: string
  locked_at: number
  candidate_count: number
  last_collected_at: number
  last_error: string
  stale: boolean
}

export interface ChannelRuntimeStatusResponse {
  enabled: boolean
  saturation_lock_enabled: boolean
  length_routing_enabled: boolean
  scrape_interval_seconds: number
  items: ChannelRuntimeStatusItem[]
}

export interface ChannelRuntimeLimitItem {
  channel_id: number
  channel_name: string
  max_batch: number
  source: string
  peak_running: number
  updated_at: number
}

// 按时间桶聚合的运行时趋势点（后端已按 bucket 对齐）
export interface ChannelRuntimeSeriesItem {
  channel_id: number
  channel_name: string
  model_names: string
  bucket: number
  avg_running: number
  max_running: number
  avg_waiting: number
  max_waiting: number
  max_batch: number
  // 桶内 Running + Waiting 的峰值（整数观测值）；avg_requests 是瞬时均值，会出现小数
  avg_requests: number
  max_requests: number
  prompt_throughput: number
  generation_throughput: number
  sample_count: number
}

// 渠道轻量信息，用于给没有样本的渠道也渲染一张空趋势卡片
export interface ChannelRuntimeChannelItem {
  id: number
  name: string
  models: string[]
  status: number
}

export interface ChannelRuntimeSeriesResponse extends ChannelSeriesResponse<ChannelRuntimeSeriesItem> {
  channels: ChannelRuntimeChannelItem[]
}

// 按时间桶聚合的渠道调用量点
export interface ChannelUsageSeriesItem {
  channel_id: number
  channel_name: string
  model_names: string
  bucket: number
  requests: number
  prompt_tokens: number
  completion_tokens: number
}

export interface ChannelSeriesResponse<T> {
  start_timestamp: number
  end_timestamp: number
  bucket_seconds: number
  items: T[]
}
