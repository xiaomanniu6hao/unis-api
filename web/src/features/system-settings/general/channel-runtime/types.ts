export interface ChannelRuntimeChannelItem {
  id: number
  name: string
  models: string[]
  status: number
}

export interface MaxBatchOverride {
  channel_name: string
  max_batch: number
}

export interface ChannelLengthRoute {
  model_name: string
  channel_name: string
  min_length: number
  max_length: number
  enabled: boolean
}

export interface ChannelRuntimeSettings {
  'channel_runtime_setting.enabled': boolean
  'channel_runtime_setting.scrape_interval_seconds': number
  'channel_runtime_setting.scrape_timeout_seconds': number
  'channel_runtime_setting.sample_retention_months': number
  'channel_runtime_setting.default_max_batch': number
  'channel_runtime_setting.max_batch_overrides': string
  'channel_runtime_setting.saturation_lock_enabled': boolean
  'channel_runtime_setting.saturation_waiting_samples': number
  'channel_runtime_setting.saturation_release_samples': number
  'channel_runtime_setting.saturation_max_lock_seconds': number
  'channel_runtime_setting.length_routing_enabled': boolean
  'channel_runtime_setting.length_routing_fallback': boolean
  'channel_runtime_setting.length_routing_rules': string
}
