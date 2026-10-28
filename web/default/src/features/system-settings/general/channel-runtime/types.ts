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

export interface MaxBatchOverride {
  channel_name: string
  max_batch: number
}

export interface ChannelLengthSegment {
  min_length: number
  max_length: number
  channel_ids: number[]
}

export interface ChannelLengthRouteRule {
  name: string
  model_regex: string[]
  segments: ChannelLengthSegment[]
  enabled: boolean
}

export interface ChannelRuntimeSettings {
  'channel_runtime_setting.enabled': boolean
  'channel_runtime_setting.scrape_interval_seconds': number
  'channel_runtime_setting.scrape_timeout_ms': number
  'channel_runtime_setting.sample_retention_hours': number
  'channel_runtime_setting.default_max_batch': number
  'channel_runtime_setting.scrape_only_channels': string
  'channel_runtime_setting.max_batch_overrides': string
  'channel_runtime_setting.saturation_lock_enabled': boolean
  'channel_runtime_setting.saturation_waiting_samples': number
  'channel_runtime_setting.saturation_release_samples': number
  'channel_runtime_setting.saturation_max_lock_seconds': number
  'channel_runtime_setting.length_routing_enabled': boolean
  'channel_runtime_setting.length_routing_fallback': boolean
  'channel_runtime_setting.length_routing_rules': string
}
