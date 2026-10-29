package operation_setting

import (
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

// ChannelMaxBatchOverride 按渠道名称手工指定最大 batch 并发数。
// 该列表同时充当采集白名单：只有列在这里的渠道才会被抓取 /metrics。
type ChannelMaxBatchOverride struct {
	ChannelName string `json:"channel_name"`
	MaxBatch    int    `json:"max_batch"`
}

// ChannelLengthRoute 把「模型 + 输入长度区间」映射到单个渠道。
// MinLength 为闭区间下界，MaxLength 为开区间上界，MaxLength <= 0 表示无上界，
// 因此 [0, 0] 就是「该模型的全部请求都走这个渠道」。
type ChannelLengthRoute struct {
	ModelName   string `json:"model_name"`
	ChannelName string `json:"channel_name"`
	MinLength   int    `json:"min_length"`
	MaxLength   int    `json:"max_length"`
	Enabled     bool   `json:"enabled"`
}

// ChannelRuntimeSetting 是渠道运行时监控 / 饱和锁 / 长度分流的总开关配置。
// 未启用采集（Enabled=false）时调度完全走原有逻辑，不读运行时数据。
type ChannelRuntimeSetting struct {
	// 采集
	Enabled               bool `json:"enabled"`
	ScrapeIntervalSeconds int  `json:"scrape_interval_seconds"`
	ScrapeTimeoutSeconds  int  `json:"scrape_timeout_seconds"`
	SampleRetentionMonths int  `json:"sample_retention_months"`

	// 最大 batch：按渠道名配置，同时决定采集范围（配了就采集，没配就不采集）
	DefaultMaxBatch   int                       `json:"default_max_batch"`
	MaxBatchOverrides []ChannelMaxBatchOverride `json:"max_batch_overrides"`

	// 饱和锁
	SaturationLockEnabled    bool `json:"saturation_lock_enabled"`
	SaturationWaitingSamples int  `json:"saturation_waiting_samples"`
	SaturationReleaseSamples int  `json:"saturation_release_samples"`
	SaturationMaxLockSeconds int  `json:"saturation_max_lock_seconds"`

	// 长度分流（长度单位固定为估算 prompt token）
	LengthRoutingEnabled  bool                 `json:"length_routing_enabled"`
	LengthRoutingFallback bool                 `json:"length_routing_fallback"`
	LengthRoutingRules    []ChannelLengthRoute `json:"length_routing_rules"`
}

const (
	// DefaultChannelMaxBatch 是新增渠道时预填的最大 batch。
	DefaultChannelMaxBatch = 4
	// DefaultScrapeIntervalSeconds 是默认采集间隔（秒）。
	DefaultScrapeIntervalSeconds = 10
	// DefaultScrapeTimeoutSeconds 是单次抓取 metrics 的默认超时（秒）。
	DefaultScrapeTimeoutSeconds = 3
	// MaxScrapeTimeoutSeconds 是抓取超时的上限（秒）。
	MaxScrapeTimeoutSeconds = 30
	// DefaultSampleRetentionMonths 默认保留 12 个月（一年）运行时样本。
	DefaultSampleRetentionMonths = 12
)

var channelRuntimeSetting = ChannelRuntimeSetting{
	Enabled:                  false,
	ScrapeIntervalSeconds:    DefaultScrapeIntervalSeconds,
	ScrapeTimeoutSeconds:     DefaultScrapeTimeoutSeconds,
	SampleRetentionMonths:    DefaultSampleRetentionMonths,
	DefaultMaxBatch:          DefaultChannelMaxBatch,
	MaxBatchOverrides:        []ChannelMaxBatchOverride{},
	SaturationLockEnabled:    true,
	SaturationWaitingSamples: 3,
	SaturationReleaseSamples: 1,
	SaturationMaxLockSeconds: 300,
	LengthRoutingEnabled:     false,
	LengthRoutingFallback:    true,
	LengthRoutingRules:       []ChannelLengthRoute{},
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
		s.ScrapeIntervalSeconds = DefaultScrapeIntervalSeconds
	}
	if s.ScrapeIntervalSeconds > 300 {
		s.ScrapeIntervalSeconds = 300
	}
	if s.ScrapeTimeoutSeconds < 1 {
		s.ScrapeTimeoutSeconds = DefaultScrapeTimeoutSeconds
	}
	if s.ScrapeTimeoutSeconds > MaxScrapeTimeoutSeconds {
		s.ScrapeTimeoutSeconds = MaxScrapeTimeoutSeconds
	}
	if s.SampleRetentionMonths < 1 {
		s.SampleRetentionMonths = DefaultSampleRetentionMonths
	}
	if s.SampleRetentionMonths > DefaultSampleRetentionMonths {
		s.SampleRetentionMonths = DefaultSampleRetentionMonths
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
	// 区间按 MinLength 升序，方便人读也方便匹配时提前收敛
	sort.SliceStable(s.LengthRoutingRules, func(a, b int) bool {
		return s.LengthRoutingRules[a].MinLength < s.LengthRoutingRules[b].MinLength
	})
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

// IsChannelCollected 判断渠道是否在采集范围内：只有配了最大 batch 的渠道才采集。
func (s *ChannelRuntimeSetting) IsChannelCollected(channelName string) bool {
	return s.LookupMaxBatchOverride(channelName) > 0
}
