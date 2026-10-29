package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
)

func TestMatchLengthRoutePrefersMostSpecificRange(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		LengthRoutingRules: []operation_setting.ChannelLengthRoute{
			{ModelName: "glm-5", ChannelName: "glm-disabled", MinLength: 0, MaxLength: 0, Enabled: false},
			{ModelName: "glm-5", ChannelName: "glm-short", MinLength: 0, MaxLength: 0, Enabled: true},
			{ModelName: "glm-5", ChannelName: "glm-long", MinLength: 10000, MaxLength: 0, Enabled: true},
			{ModelName: "glm-5", ChannelName: "glm-mid", MinLength: 1000, MaxLength: 2000, Enabled: true},
		},
	}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	// 没有更具体的区间命中时走 [0,0] 兜底
	assert.Equal(t, "glm-short", matchLengthRoute(setting, "glm-5", 0).ChannelName)
	assert.Equal(t, "glm-short", matchLengthRoute(setting, "glm-5", 999).ChannelName)
	// 落在 [1000,2000) 内取下界更大的规则
	assert.Equal(t, "glm-mid", matchLengthRoute(setting, "glm-5", 1000).ChannelName)
	assert.Equal(t, "glm-mid", matchLengthRoute(setting, "glm-5", 1999).ChannelName)
	// 上界开区间，2000 不再命中 mid
	assert.Equal(t, "glm-short", matchLengthRoute(setting, "glm-5", 2000).ChannelName)
	// 长上下文优先走专用渠道
	assert.Equal(t, "glm-long", matchLengthRoute(setting, "glm-5", 10000).ChannelName)
	assert.Equal(t, "glm-long", matchLengthRoute(setting, "glm-5", 500000).ChannelName)

	// 模型名精确匹配（大小写不敏感），其他模型不分流
	assert.Equal(t, "glm-long", matchLengthRoute(setting, "GLM-5", 20000).ChannelName)
	assert.Nil(t, matchLengthRoute(setting, "qwen3", 100))
}

func TestMatchLengthRouteEmptyModelNameMatchesAll(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		LengthRoutingRules: []operation_setting.ChannelLengthRoute{
			{ModelName: "", ChannelName: "shared", MinLength: 0, MaxLength: 0, Enabled: true},
			{ModelName: "glm-5", ChannelName: "", MinLength: 0, MaxLength: 0, Enabled: true},
		},
	}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	// 模型名留空表示对所有模型生效；渠道名为空的行直接忽略
	assert.Equal(t, "shared", matchLengthRoute(setting, "anything", 10).ChannelName)
}

func TestNormalizeChannelRuntimeSettingDefaults(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{}
	operation_setting.NormalizeChannelRuntimeSetting(setting)

	assert.Equal(t, 10, setting.ScrapeIntervalSeconds)
	// 采集超时按秒，默认 3 秒
	assert.Equal(t, operation_setting.DefaultScrapeTimeoutSeconds, setting.ScrapeTimeoutSeconds)
	assert.Equal(t, 3, setting.ScrapeTimeoutSeconds)
	// 样本默认保留 12 个月（一年）
	assert.Equal(t, operation_setting.DefaultSampleRetentionMonths, setting.SampleRetentionMonths)
	assert.Equal(t, 12, setting.SampleRetentionMonths)
	// 最大 batch 默认 4
	assert.Equal(t, 4, setting.DefaultMaxBatch)
	assert.Equal(t, 3, setting.SaturationWaitingSamples)
	assert.Equal(t, 1, setting.SaturationReleaseSamples)
}

func TestIsChannelCollectedFollowsMaxBatchOverrides(t *testing.T) {
	setting := &operation_setting.ChannelRuntimeSetting{
		MaxBatchOverrides: []operation_setting.ChannelMaxBatchOverride{
			{ChannelName: "glm-h20-01", MaxBatch: 16},
			{ChannelName: "broken", MaxBatch: 0},
		},
	}
	// 配了最大 batch 的渠道才采集，未配置（或值非法）的一律跳过
	assert.True(t, setting.IsChannelCollected("glm-h20-01"))
	assert.True(t, setting.IsChannelCollected("GLM-H20-01"))
	assert.False(t, setting.IsChannelCollected("glm-h20-02"))
	assert.False(t, setting.IsChannelCollected("broken"))
}
