import type { TimeGranularity } from '@/lib/time'

// 渠道分析的时间范围：当天 / 当周 / 当月 / 自定义。
export type ChannelRangePreset = 'day' | 'week' | 'month' | 'custom'

export interface ChannelTimeRange {
  preset: ChannelRangePreset
  start: Date
  end: Date
}

const DAY_SECONDS = 86400

function startOfDay(date: Date): Date {
  const result = new Date(date)
  result.setHours(0, 0, 0, 0)
  return result
}

// 当周从周一 00:00 开始（getDay 里周日是 0，需要换算成 7）
function startOfWeek(date: Date): Date {
  const result = startOfDay(date)
  const weekday = result.getDay() || 7
  result.setDate(result.getDate() - (weekday - 1))
  return result
}

function startOfMonth(date: Date): Date {
  const result = startOfDay(date)
  result.setDate(1)
  return result
}

// 自定义范围为空时回退到最近 24 小时，保证图表总有一个可查询的窗口。
export function resolveChannelTimeRange(
  preset: ChannelRangePreset,
  custom?: { start?: Date; end?: Date },
  now: Date = new Date()
): ChannelTimeRange {
  if (preset === 'custom') {
    const end = custom?.end ?? now
    const start = custom?.start ?? new Date(end.getTime() - DAY_SECONDS * 1000)
    return { preset, start, end }
  }
  if (preset === 'week') return { preset, start: startOfWeek(now), end: now }
  if (preset === 'month') return { preset, start: startOfMonth(now), end: now }
  return { preset: 'day', start: startOfDay(now), end: now }
}

export function channelRangeSeconds(range: ChannelTimeRange): number {
  const seconds = Math.floor(
    (range.end.getTime() - range.start.getTime()) / 1000
  )
  return Math.max(seconds, 60)
}

export function channelRangeParams(range: ChannelTimeRange): {
  start_timestamp: number
  end_timestamp: number
} {
  return {
    start_timestamp: Math.floor(range.start.getTime() / 1000),
    end_timestamp: Math.floor(range.end.getTime() / 1000),
  }
}

// 调用分析的桶与模型调用分析保持一致：当天按小时，当周/当月按天；
// 自定义范围窗口很短时也退回按小时，避免整窗只剩一个柱子。
function usageUsesHourBucket(range: ChannelTimeRange): boolean {
  if (range.preset === 'day') return true
  if (range.preset === 'custom') {
    return channelRangeSeconds(range) <= 2 * DAY_SECONDS
  }
  return false
}

export function channelUsageBucketSeconds(range: ChannelTimeRange): number {
  return usageUsesHourBucket(range) ? 3600 : DAY_SECONDS
}

export function channelUsageGranularity(
  range: ChannelTimeRange
): TimeGranularity {
  return usageUsesHourBucket(range) ? 'hour' : 'day'
}

// 运行时趋势点数更多，按范围压到约 300 个桶（5 分钟 ~ 2 小时）。
export function channelRuntimeBucketSeconds(range: ChannelTimeRange): number {
  const seconds = channelRangeSeconds(range)
  if (range.preset === 'day') return 300
  if (range.preset === 'week') return 1800
  if (range.preset === 'month') return 7200
  const raw = Math.floor(seconds / 300 / 60) * 60
  return Math.min(Math.max(raw, 60), 7200)
}
