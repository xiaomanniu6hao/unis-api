package operation_setting

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// ChannelMaxBatchOverride 按渠道名称手工指定最大 batch 并发数。
// 一个渠道对应一个 vLLM 实例、一个模型，所以不需要再绑定模型名。
type ChannelMaxBatchOverride struct {
	ChannelName string `json:"channel_name"`
	MaxBatch    int    `json:"max_batch"`
}

// ChannelLengthSegment 是一段输入长度区间及其允许调度的渠道集合。
// MinLength 为闭区间下界，MaxLength 为开区间上界，MaxLength <= 0 表示无上界。
type ChannelLengthSegment struct {
	MinLength  int   `json:"min_length"`
	MaxLength  int   `json:"max_length"`
	ChannelIds []int `json:"channel_ids"`
}

// ChannelLengthRouteRule 把某个模型（正则匹配）的输入长度映射到不同渠道集合。
type ChannelLengthRouteRule struct {
	Name       string                 `json:"name"`
	ModelRegex []string               `json:"model_regex"`
	Segments   []ChannelLengthSegment `json:"segments"`
	Enabled    bool                   `json:"enabled"`
}

// ChannelRuntimeSetting 是渠道运行时监控 / 饱和锁 / 长度分流的总开关配置。
// 未启用采集（Enabled=false）时调度完全走原有逻辑，不读运行时数据。
type ChannelRuntimeSetting struct {
	// 采集
	Enabled               bool `json:"enabled"`
	ScrapeIntervalSeconds int  `json:"scrape_interval_seconds"`
	ScrapeTimeoutMs       int  `json:"scrape_timeout_ms"`
	SampleRetentionHours  int  `json:"sample_retention_hours"`
	// ScrapeOnlyChannels 非空时只采集这些渠道，避免对官方 API 渠道发无效请求。
	ScrapeOnlyChannels []int `json:"scrape_only_channels"`

	// 最大 batch：按渠道名覆盖，未命中用 DefaultMaxBatch
	DefaultMaxBatch   int                       `json:"default_max_batch"`
	MaxBatchOverrides []ChannelMaxBatchOverride `json:"max_batch_overrides"`

	// 饱和锁
	SaturationLockEnabled    bool `json:"saturation_lock_enabled"`
	SaturationWaitingSamples int  `json:"saturation_waiting_samples"`
	SaturationReleaseSamples int  `json:"saturation_release_samples"`
	SaturationMaxLockSeconds int  `json:"saturation_max_lock_seconds"`

	// 长度分流（长度单位固定为估算 prompt token）
	LengthRoutingEnabled  bool                     `json:"length_routing_enabled"`
	LengthRoutingFallback bool                     `json:"length_routing_fallback"`
	LengthRoutingRules    []ChannelLengthRouteRule `json:"length_routing_rules"`
}

const (
	// DefaultChannelMaxBatch 是拿不到任何上限信息时的兜底并发数。
	DefaultChannelMaxBatch = 4
	// DefaultSampleRetentionHours 默认保留一年运行时样本。
	DefaultSampleRetentionHours = 24 * 365
)

var channelRuntimeSetting = ChannelRuntimeSetting{
	Enabled:                  false,
	ScrapeIntervalSeconds:    5,
	ScrapeTimeoutMs:          3000,
	SampleRetentionHours:     DefaultSampleRetentionHours,
	ScrapeOnlyChannels:       []int{},
	DefaultMaxBatch:          DefaultChannelMaxBatch,
	MaxBatchOverrides:        []ChannelMaxBatchOverride{},
	SaturationLockEnabled:    true,
	SaturationWaitingSamples: 3,
	SaturationReleaseSamples: 1,
	SaturationMaxLockSeconds: 300,
	LengthRoutingEnabled:     false,
	LengthRoutingFallback:    true,
	LengthRoutingRules: []ChannelLengthRouteRule{
		{
			Name:       "example long context split",
			ModelRegex: []string{"^$"},
			Enabled:    false,
			Segments: []ChannelLengthSegment{
				{MinLength: 0, MaxLength: 10000, ChannelIds: []int{}},
				{MinLength: 10000, MaxLength: 0, ChannelIds: []int{}},
			},
		},
	},
}

func init() {
	config.GlobalConfig.Register("channel_runtime_setting", &channelRuntimeSetting)
}

func GetChannelRuntimeSetting() *ChannelRuntimeSetting {
	return &channelRuntimeSetting
}

// NormalizeChannelRuntimeSetting 把越界配置拉回安全范围，避免 0 值把采集/判定打穿。
func NormalizeChannelRuntimeSetting(s *ChannelRuntimeSetting) {
	if s == nil {
		return
	}
	if s.ScrapeIntervalSeconds < 1 {
		s.ScrapeIntervalSeconds = 5
	}
	if s.ScrapeIntervalSeconds > 300 {
		s.ScrapeIntervalSeconds = 300
	}
	if s.ScrapeTimeoutMs < 200 {
		s.ScrapeTimeoutMs = 3000
	}
	if s.ScrapeTimeoutMs > 30000 {
		s.ScrapeTimeoutMs = 30000
	}
	if s.SampleRetentionHours < 1 {
		s.SampleRetentionHours = DefaultSampleRetentionHours
	}
	if s.SampleRetentionHours > DefaultSampleRetentionHours {
		s.SampleRetentionHours = DefaultSampleRetentionHours
	}
	if s.DefaultMaxBatch < 1 {
		s.DefaultMaxBatch = DefaultChannelMaxBatch
	}
	if s.SaturationWaitingSamples < 1 {
		s.SaturationWaitingSamples = 3
	}
	if s.SaturationReleaseSamples < 1 {
		s.SaturationReleaseSamples = 1
	}
	if s.SaturationMaxLockSeconds < 0 {
		s.SaturationMaxLockSeconds = 0
	}
	for i := range s.LengthRoutingRules {
		rule := &s.LengthRoutingRules[i]
		sort.SliceStable(rule.Segments, func(a, b int) bool {
			return rule.Segments[a].MinLength < rule.Segments[b].MinLength
		})
	}
}

// LookupMaxBatchOverride 按渠道名返回手工覆盖值，未配置时返回 0。
func (s *ChannelRuntimeSetting) LookupMaxBatchOverride(channelName string) int {
	if s == nil {
		return 0
	}
	channelName = strings.TrimSpace(channelName)
	for _, override := range s.MaxBatchOverrides {
		if override.MaxBatch <= 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(override.ChannelName), channelName) {
			continue
		}
		return override.MaxBatch
	}
	return 0
}

// ShouldScrapeChannel 判断渠道是否在采集范围内。
func (s *ChannelRuntimeSetting) ShouldScrapeChannel(channelId int) bool {
	if s == nil || len(s.ScrapeOnlyChannels) == 0 {
		return true
	}
	for _, id := range s.ScrapeOnlyChannels {
		if id == channelId {
			return true
		}
	}
	return false
}
