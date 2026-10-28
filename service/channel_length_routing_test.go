package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
)

func TestMatchLengthRouteRuleAndSegments(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		LengthRoutingRules: []operation_setting.ChannelLengthRouteRule{
			{
				Name:       "disabled rule",
				ModelRegex: []string{".*"},
				Enabled:    false,
				Segments:   []operation_setting.ChannelLengthSegment{{MinLength: 0, MaxLength: 0, ChannelIds: []int{1}}},
			},
			{
				Name:       "glm split",
				ModelRegex: []string{"^GLM5.*$"},
				Enabled:    true,
				Segments: []operation_setting.ChannelLengthSegment{
					{MinLength: 10000, MaxLength: 0, ChannelIds: []int{2, 3, 4}},
					{MinLength: 0, MaxLength: 10000, ChannelIds: []int{1}},
				},
			},
		},
	}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	rule := matchLengthRouteRule(setting, "GLM5.1")
	assert.NotNil(t, rule)
	assert.Equal(t, "glm split", rule.Name)

	assert.Nil(t, matchLengthRouteRule(setting, "qwen3"), "未命中正则时不应分流")

	// 区间按 [min, max) 判定，归一化后已按 MinLength 升序
	segment, ok := pickLengthSegment(rule, 0)
	assert.True(t, ok)
	assert.Equal(t, []int{1}, segment.ChannelIds)

	segment, ok = pickLengthSegment(rule, 9999)
	assert.True(t, ok)
	assert.Equal(t, []int{1}, segment.ChannelIds)

	segment, ok = pickLengthSegment(rule, 10000)
	assert.True(t, ok)
	assert.Equal(t, []int{2, 3, 4}, segment.ChannelIds)

	segment, ok = pickLengthSegment(rule, 500000)
	assert.True(t, ok)
	assert.Equal(t, []int{2, 3, 4}, segment.ChannelIds)
}

func TestNormalizeChannelRuntimeSettingDefaults(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	assert.Equal(t, 5, setting.ScrapeIntervalSeconds)
	assert.Equal(t, 3000, setting.ScrapeTimeoutMs)
	// 样本默认保留一年
	assert.Equal(t, operation_setting.DefaultSampleRetentionHours, setting.SampleRetentionHours)
	assert.Equal(t, 8760, setting.SampleRetentionHours)
	// 最大 batch 默认 4
	assert.Equal(t, 4, setting.DefaultMaxBatch)
	assert.Equal(t, 3, setting.SaturationWaitingSamples)
	assert.Equal(t, 1, setting.SaturationReleaseSamples)
}
